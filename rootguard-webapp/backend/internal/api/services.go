// =====================================================
// File: backend/internal/api/services.go
// Project: RootGuard WebApp
// Purpose: Return detected services as JSON
// =====================================================

package api

import (
	"net/http"

	"github.com/foxly-it/rootguard-webapp/backend/internal/coreclient"
)

// -----------------------------------------------------
// HandleServices
//
// GET /api/services
//
// Returns all detected services from the service
// detection engine.
// -----------------------------------------------------

func HandleServices(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	// The router (see httpapi.NewRouter) already gates this path to GET
	// before calling in.
	proxyFixed(w, r, http.StatusInternalServerError, core.Services)
}

func HandleServiceLogs(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	logs, err := core.ServiceLogs(r.Context(), r.PathValue("name"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, http.StatusOK, logs)
}
