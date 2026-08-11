package api

import (
	"encoding/json"
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

func HandleBackupStatus(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	status, err := core.BackupStatus(r.Context())
	if err != nil {
		writeCoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func HandlePutBackupSettings(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10))
	decoder.DisallowUnknownFields()
	var request struct {
		RetentionPerService int `json:"retention_per_service"`
	}
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	status, err := core.SetBackupRetention(r.Context(), request.RetentionPerService)
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
