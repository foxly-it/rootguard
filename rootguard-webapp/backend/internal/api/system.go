// =====================================================
// File: backend/internal/api/system.go
// Project: RootGuard WebApp
// Purpose: System overview API handler
//
// Endpoint:
//
// GET /api/system
//
// =====================================================

package api

import (
	"net/http"

	"github.com/foxly-it/rootguard-webapp/backend/internal/coreclient"
)

// =====================================================
// HandleSystem
//
// Returns aggregated Docker system stats.
// =====================================================

func HandleSystem(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	stats, err := core.System(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}
