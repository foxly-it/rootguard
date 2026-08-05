package api

import (
	"net/http"

	"github.com/foxly-it/rootguard-webapp/backend/internal/coreclient"
)

func HandleUpdateStatus(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	status, err := core.UpdateStatus(r.Context())
	if err != nil {
		writeCoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func HandleUpdateCheck(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	status, err := core.CheckUpdates(r.Context())
	if err != nil {
		writeCoreError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, status)
}

func HandleUpdateService(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	status, err := core.UpdateService(r.Context(), r.PathValue("name"))
	if err != nil {
		writeCoreError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, status)
}
