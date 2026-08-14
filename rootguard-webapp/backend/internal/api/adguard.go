package api

import (
	"encoding/json"
	"net/http"

	"github.com/foxly-it/rootguard-webapp/backend/internal/coreclient"
)

func HandleGetAdGuardStatus(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	proxyFixed(w, r, http.StatusBadGateway, core.AdGuardStatus)
}

func HandleBootstrapAdGuard(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	proxyFixed(w, r, http.StatusBadGateway, core.BootstrapAdGuard)
}

func HandleGetAdGuardFilterReport(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	proxyFixed(w, r, http.StatusBadGateway, core.AdGuardFilterReport)
}

func HandleSetAdGuardFiltering(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	var input struct {
		Enabled bool `json:"enabled"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	status, err := core.SetAdGuardFiltering(r.Context(), input.Enabled)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, http.StatusOK, status)
}
