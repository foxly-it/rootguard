// =====================================================
// File: backend/internal/api/service.go
// Project: RootGuard WebApp
// Purpose: Handle service control actions
//
// Endpoint:
//
// POST /api/service/{name}/{action}
//
// Examples:
//
// POST /api/service/adguard/start
// POST /api/service/unbound/restart
//
// =====================================================

package api

import (
	"net/http"
	"strings"

	"github.com/foxly-it/rootguard-webapp/backend/internal/coreclient"
)

// -----------------------------------------------------
// HandleServiceAction
// -----------------------------------------------------

func HandleServiceAction(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/service/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		http.Error(w, "Invalid service path", http.StatusBadRequest)
		return
	}
	serviceName, action := parts[0], parts[1]
	if action != "start" && action != "stop" && action != "restart" {
		http.Error(w, "Invalid action", http.StatusBadRequest)
		return
	}

	response, err := core.ServiceAction(r.Context(), serviceName, action)
	if err != nil {
		// Found in a v1.0.0 correctness review: every other Core-proxying
		// handler in this package already uses writeCoreError (see
		// unbound.go) to propagate Core's own 4xx status instead of
		// flattening it - an unknown service name or a conflicting action
		// Core itself rejects with 400/404/409 used to come back as a
		// bare 500 here, misrepresenting an operator's own bad input as a
		// server-side failure.
		writeCoreError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, response)
}
