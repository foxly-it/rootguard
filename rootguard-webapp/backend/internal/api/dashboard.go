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
	proxyFixed(w, r, http.StatusInternalServerError, core.Dashboard)
}
