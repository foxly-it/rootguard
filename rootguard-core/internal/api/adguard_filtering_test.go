package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/foxly-it/rootguard-core/internal/adguard"
)

// A dummy manager is safe for these cases: every one of them is rejected by
// the handler's own validation before it ever calls into the manager.
func dummyAdGuardManager() *adguard.Manager {
	return adguard.NewManager("", "", "", "", "")
}

func TestSetAdGuardFilteringHandlerRejectsMissingEnabled(t *testing.T) {
	// {} used to decode to Enabled=false with no way to tell it apart from
	// an explicit {"enabled":false} - found via code review, same class of
	// bug as the protection endpoint.
	handler := setAdGuardFilteringHandler(dummyAdGuardManager())
	request := httptest.NewRequest(http.MethodPost, "/api/adguard/filtering", strings.NewReader(`{}`))
	response := httptest.NewRecorder()
	handler(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected an empty body to be rejected with 400, got %d", response.Code)
	}
}

func TestSetAdGuardFilteringHandlerRejectsTrailingData(t *testing.T) {
	// A bare Decode call silently ignores anything after the first JSON
	// value - found via code review.
	handler := setAdGuardFilteringHandler(dummyAdGuardManager())
	request := httptest.NewRequest(http.MethodPost, "/api/adguard/filtering", strings.NewReader(`{"enabled":true}{"enabled":false}`))
	response := httptest.NewRecorder()
	handler(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected trailing JSON data to be rejected with 400, got %d", response.Code)
	}
}

func TestSetAdGuardProtectionHandlerRejectsTrailingData(t *testing.T) {
	handler := setAdGuardProtectionHandler(dummyAdGuardManager())
	request := httptest.NewRequest(http.MethodPost, "/api/adguard/protection", strings.NewReader(`{"enabled":true,"duration_seconds":0}{"extra":true}`))
	response := httptest.NewRecorder()
	handler(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected trailing JSON data to be rejected with 400, got %d", response.Code)
	}
}
