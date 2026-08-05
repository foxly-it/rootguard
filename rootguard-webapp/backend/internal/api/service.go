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

	"github.com/foxly-it/rootguard-webapp/backend/internal/coreclient"
)

// -----------------------------------------------------
// HandleServiceAction
// -----------------------------------------------------

func HandleServiceAction(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	serviceName, action, ok := parseServiceActionPath(r.URL.Path)

	if !ok {
		http.Error(w, "Invalid service path", http.StatusBadRequest)
		return
	}
	if action != "start" && action != "stop" && action != "restart" {
		http.Error(w, "Invalid action", http.StatusBadRequest)
		return
	}

	response, err := core.ServiceAction(r.Context(), serviceName, action)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, response)
}
