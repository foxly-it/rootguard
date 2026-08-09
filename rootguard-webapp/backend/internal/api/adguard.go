package api

import (
	"encoding/json"
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
