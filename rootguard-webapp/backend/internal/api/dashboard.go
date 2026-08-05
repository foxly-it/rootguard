// =====================================================
// File: backend/internal/api/dashboard.go
// Purpose: Dashboard API endpoint
// =====================================================

package api

import (
	"net/http"

	"github.com/foxly-it/rootguard-webapp/backend/internal/coreclient"
)

// -----------------------------------------------------
// HandleDashboard
// HTTP endpoint returning dashboard statistics
// -----------------------------------------------------

func HandleDashboard(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	stats, err := core.Dashboard(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}
