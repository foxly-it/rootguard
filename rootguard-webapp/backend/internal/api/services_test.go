package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/foxly-it/rootguard-webapp/backend/internal/coreclient"
)

// TestHandleServiceLogsPropagatesCoresOwnStatusCode is the regression test
// for a follow-up review finding: HandleServiceLogs still flattened every
// Core error - including a 404/400 for an unknown service name Core
// itself rejects - into a bare 502, the same bug HandleServiceAction
// already had fixed (see service_test.go's identical test).
func TestHandleServiceLogsPropagatesCoresOwnStatusCode(t *testing.T) {
	coreServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":"unknown service"}`, http.StatusNotFound)
	}))
	defer coreServer.Close()
	core := coreclient.New(coreServer.URL, "test-token")

	request := httptest.NewRequest(http.MethodGet, "/api/services/bogus/logs", nil)
	request.SetPathValue("name", "bogus")
	response := httptest.NewRecorder()
	HandleServiceLogs(response, request, core)
	if response.Code != http.StatusNotFound {
		t.Fatalf("expected Core's own 404 to be propagated, got %d: %s", response.Code, response.Body.String())
	}
}
