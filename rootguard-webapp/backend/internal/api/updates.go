package api

import (
	"io"
	"net/http"
	"time"

	"github.com/foxly-it/rootguard-webapp/backend/internal/coreclient"
)

func HandleUpdateStatus(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	proxyCore(w, r, http.StatusOK, core.UpdateStatus)
}

func HandleBackupStatus(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	proxyCore(w, r, http.StatusOK, core.BackupStatus)
}

func HandlePutBackupSettings(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	type putBackupSettingsRequest struct {
		RetentionPerService int `json:"retention_per_service"`
	}
	defer r.Body.Close()
	request, err := decodeJSON[putBackupSettingsRequest](w, r, 4<<10)
	if err != nil {
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
	proxyCore(w, r, http.StatusOK, core.CleanupPreview)
}

func HandleRunCleanup(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	proxyCore(w, r, http.StatusOK, core.RunCleanup)
}

func HandleBackupExport(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	type backupExportRequest struct {
		Passphrase string `json:"passphrase"`
	}
	defer r.Body.Close()
	request, err := decodeJSON[backupExportRequest](w, r, 4<<10)
	if err != nil {
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
	_ = http.NewResponseController(w).SetReadDeadline(time.Now().Add(10 * time.Minute))
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
	proxyCore(w, r, http.StatusAccepted, core.CheckUpdates)
}

func HandleUpdateService(w http.ResponseWriter, r *http.Request, core *coreclient.Client) {
	status, err := core.UpdateService(r.Context(), r.PathValue("name"))
	if err != nil {
		writeCoreError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, status)
}
