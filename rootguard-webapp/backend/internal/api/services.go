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
		// Found in review: same bug already fixed in HandleServiceAction
		// (see service.go) - a bare 502 misrepresented an unknown service
		// name (Core's own 400/404) as a backend failure instead of
		// propagating Core's real status via writeCoreError.
		writeCoreError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, logs)
}
