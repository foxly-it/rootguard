package api

import (
	"net/http"

	"github.com/foxly-it/rootguard-webapp/backend/internal/coreclient"
)

func HandleGetAdGuardStatus(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	status, err := core.AdGuardStatus(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func HandleBootstrapAdGuard(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	status, err := core.BootstrapAdGuard(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func HandleGetAdGuardFilterReport(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	report, err := core.AdGuardFilterReport(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, http.StatusOK, report)
}
