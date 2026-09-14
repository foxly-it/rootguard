package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/foxly-it/rootguard-webapp/backend/internal/coreclient"
)

// TestHandleServiceActionPropagatesCoresOwnStatusCode is the regression
// test for a v1.0.0 correctness review finding: every other Core-proxying
// handler in this package uses writeCoreError to propagate Core's own 4xx
// status (see unbound.go and its identical tests), but HandleServiceAction
// still flattened every error - including a 404 for an unknown service
// name Core itself rejects - into a bare 500, misrepresenting an
// operator's own bad input as a server-side failure.
func TestHandleServiceActionPropagatesCoresOwnStatusCode(t *testing.T) {
	coreServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":"unknown service"}`, http.StatusNotFound)
	}))
	defer coreServer.Close()
	core := coreclient.New(coreServer.URL, "test-token")

	request := httptest.NewRequest(http.MethodPost, "/api/service/bogus/start", nil)
	response := httptest.NewRecorder()
	HandleServiceAction(response, request, core)
	if response.Code != http.StatusNotFound {
		t.Fatalf("expected Core's own 404 to be propagated, got %d: %s", response.Code, response.Body.String())
	}
}

func TestHandleServiceActionRejectsInvalidPath(t *testing.T) {
	core := coreclient.New("http://unused.invalid", "test-token")
	request := httptest.NewRequest(http.MethodPost, "/api/service/onlyname", nil)
	response := httptest.NewRecorder()
	HandleServiceAction(response, request, core)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a malformed service path, got %d: %s", response.Code, response.Body.String())
	}
}

func TestHandleServiceActionRejectsUnknownAction(t *testing.T) {
	core := coreclient.New("http://unused.invalid", "test-token")
	request := httptest.NewRequest(http.MethodPost, "/api/service/adguard/dance", nil)
	response := httptest.NewRecorder()
	HandleServiceAction(response, request, core)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for an unrecognized action, got %d: %s", response.Code, response.Body.String())
	}
}
