package api

import (
	"encoding/json"
	"net/http"

	"github.com/foxly-it/rootguard-webapp/backend/internal/coreclient"
)

func HandleInstallationStatus(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	status, err := core.InstallationStatus(r.Context())
	if err != nil {
		writeCoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func HandleInstallationPreflight(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	config, ok := decodeInstallationConfig(w, r)
	if !ok {
		return
	}
	report, err := core.InstallationPreflight(r.Context(), config)
	if err != nil {
		writeCoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func HandleInstallationDeploy(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	config, ok := decodeInstallationConfig(w, r)
	if !ok {
		return
	}
	status, err := core.DeployInstallation(r.Context(), config)
	if err != nil {
		writeCoreError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, status)
}

func decodeInstallationConfig(w http.ResponseWriter, r *http.Request) (coreclient.InstallationConfig, bool) {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
	decoder.DisallowUnknownFields()
	var config coreclient.InstallationConfig
	if err := decoder.Decode(&config); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return coreclient.InstallationConfig{}, false
	}
	return config, true
}
