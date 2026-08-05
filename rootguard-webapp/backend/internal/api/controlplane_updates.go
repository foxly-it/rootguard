package api

import (
	"net/http"

	"github.com/foxly-it/rootguard-webapp/backend/internal/coreclient"
)

func HandleControlPlaneUpdateStatus(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	result, err := core.ControlPlaneUpdateStatus(r.Context())
	if err != nil {
		writeCoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func HandleControlPlaneUpdateCheck(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	result, err := core.CheckControlPlaneUpdates(r.Context())
	if err != nil {
		writeCoreError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, result)
}

func HandleControlPlaneUpdateInstall(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	result, err := core.InstallControlPlaneUpdates(r.Context())
	if err != nil {
		writeCoreError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, result)
}
