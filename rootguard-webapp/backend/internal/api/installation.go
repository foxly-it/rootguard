package api

import (
	"net/http"

	"github.com/foxly-it/rootguard-webapp/backend/internal/coreclient"
)

func HandleInstallationStatus(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	proxyCore(w, r, http.StatusOK, core.InstallationStatus)
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
	config, err := decodeJSON[coreclient.InstallationConfig](w, r, 8<<10)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return coreclient.InstallationConfig{}, false
	}
	return config, true
}
