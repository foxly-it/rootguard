package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/foxly-it/rootguard-webapp/backend/internal/coreclient"
)

type decodeJSONTestPayload struct {
	Name string `json:"name"`
}

func TestDecodeJSONDecodesAValidBody(t *testing.T) {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"rootguard"}`))
	value, err := decodeJSON[decodeJSONTestPayload](response, request, 1<<10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if value.Name != "rootguard" {
		t.Fatalf("expected name %q, got %q", "rootguard", value.Name)
	}
}

func TestDecodeJSONRejectsUnknownFields(t *testing.T) {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"rootguard","surprise":true}`))
	if _, err := decodeJSON[decodeJSONTestPayload](response, request, 1<<10); err == nil {
		t.Fatal("expected an unknown-field error, got nil")
	}
}

func TestDecodeJSONRejectsTrailingData(t *testing.T) {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"rootguard"}{"name":"ignored-second-document"}`))
	if _, err := decodeJSON[decodeJSONTestPayload](response, request, 1<<10); err == nil {
		t.Fatal("expected a trailing-data error, got nil")
	}
}

// TestHandleSetAdGuardFilteringRejectsTrailingData is a regression test for
// the shared decodeJSON helper (see helpers.go): every handler in this
// package used to run its own bare decoder.Decode() call, and only the two
// AdGuard handlers had a manual decoder.More() check added for it - the
// rest (this package has a dozen such handlers) had no protection at all
// against a body like {"enabled":true}{"extra":true} silently discarding
// its second half. Picks one representative handler; decodeJSON itself is
// unit-tested above and used identically everywhere else in this package.
func TestHandleSetAdGuardFilteringRejectsTrailingData(t *testing.T) {
	// Never actually reached - the handler must reject the request before
	// calling Core at all - so a server that always fails makes that
	// explicit rather than masking a bug behind a lenient fake response.
	coreServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "must not be called", http.StatusTeapot)
	}))
	defer coreServer.Close()
	core := coreclient.New(coreServer.URL, "test-token")

	request := httptest.NewRequest(http.MethodPost, "/api/adguard/filtering", strings.NewReader(`{"enabled":true}{"extra":true}`))
	response := httptest.NewRecorder()
	HandleSetAdGuardFiltering(response, request, core)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected trailing JSON data to be rejected with 400, got %d: %s", response.Code, response.Body.String())
	}
}
