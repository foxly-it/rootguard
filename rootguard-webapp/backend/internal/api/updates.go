package api

import (
	"encoding/json"
	"io"
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

func HandleCleanupPreview(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	preview, err := core.CleanupPreview(r.Context())
	if err != nil {
		writeCoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

func HandleRunCleanup(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	result, err := core.RunCleanup(r.Context())
	if err != nil {
		writeCoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func HandleBackupExport(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10))
	decoder.DisallowUnknownFields()
	var request struct {
		Passphrase string `json:"passphrase"`
	}
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	response, err := core.ExportBackup(r.Context(), request.Passphrase)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		http.Error(w, string(message), response.StatusCode)
		return
	}
	for _, header := range []string{"Content-Type", "Content-Disposition", "Cache-Control"} {
		w.Header().Set(header, response.Header.Get(header))
	}
	_, _ = io.Copy(w, response.Body)
}

func HandleBackupRestore(w http.ResponseWriter, r *http.Request, core *coreclient.Client, preview bool) {
	defer r.Body.Close()
	r.Body = http.MaxBytesReader(w, r.Body, (1<<30)+(1<<20))
	path := "/api/backups/restore"
	if preview {
		path += "/preview"
	}
	response, err := core.RestoreBackupRequest(r.Context(), path, r.Header.Get("Content-Type"), r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer response.Body.Close()
	for _, header := range []string{"Content-Type", "Cache-Control"} {
		w.Header().Set(header, response.Header.Get(header))
	}
	w.WriteHeader(response.StatusCode)
	_, _ = io.Copy(w, response.Body)
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
