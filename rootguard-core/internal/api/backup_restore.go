package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/foxly-it/rootguard-core/internal/backupexport"
	"github.com/foxly-it/rootguard-core/internal/backuprestore"
	"github.com/foxly-it/rootguard-core/internal/installer"
	"github.com/foxly-it/rootguard-core/internal/updater"
)

const restoreRequestOverhead = 1 << 20

func backupRestorePreviewHandler(manager *backuprestore.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = http.NewResponseController(w).SetReadDeadline(time.Now().Add(10 * time.Minute))
		passphrase, archive, cleanup, ok := restoreUpload(w, r)
		if !ok {
			return
		}
		defer cleanup()
		var target *installer.Config
		if value := r.FormValue("config"); value != "" {
			var config installer.Config
			if err := json.Unmarshal([]byte(value), &config); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			target = &config
		}
		preview, err := manager.Preview(r.Context(), passphrase, archive, target)
		if err != nil {
			writeRestoreError(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, preview)
	}
}

func backupRestoreHandler(manager *backuprestore.Manager, updates *updater.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = http.NewResponseController(w).SetReadDeadline(time.Now().Add(10 * time.Minute))
		passphrase, archive, cleanup, ok := restoreUpload(w, r)
		if !ok {
			return
		}
		defer cleanup()
		if r.FormValue("confirmation") != "RESTORE" {
			writeError(w, http.StatusBadRequest, errors.New("restore confirmation is required"))
			return
		}
		var config installer.Config
		if err := json.Unmarshal([]byte(r.FormValue("config")), &config); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("decode target configuration: %w", err))
			return
		}
		var status installer.Status
		err := updates.RunExclusive("Verifiziertes Vollbackup wird wiederhergestellt.", func() error {
			var restoreErr error
			status, restoreErr = manager.Restore(r.Context(), backuprestore.RestoreRequest{Passphrase: passphrase, Config: config, Archive: archive})
			return restoreErr
		})
		if err != nil {
			writeRestoreError(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, status)
	}
}

func restoreUpload(w http.ResponseWriter, r *http.Request) (string, multipart.File, func(), bool) {
	r.Body = http.MaxBytesReader(w, r.Body, backuprestore.MaxEncryptedBytes+restoreRequestOverhead)
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("parse restore upload: %w", err))
		return "", nil, func() {}, false
	}
	cleanup := func() { _ = r.MultipartForm.RemoveAll() }
	passphrase := r.FormValue("passphrase")
	if err := backupexport.ValidatePassphrase(passphrase); err != nil {
		cleanup()
		writeError(w, http.StatusBadRequest, err)
		return "", nil, func() {}, false
	}
	file, _, err := r.FormFile("archive")
	if err != nil {
		cleanup()
		writeError(w, http.StatusBadRequest, errors.New("backup archive is required"))
		return "", nil, func() {}, false
	}
	return passphrase, file, func() { _ = file.Close(); cleanup() }, true
}

func writeRestoreError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	if errors.Is(err, backuprestore.ErrInvalidBackup) {
		status = http.StatusBadRequest
	}
	if errors.Is(err, installer.ErrNotClean) || errors.Is(err, updater.ErrBusy) {
		status = http.StatusConflict
	}
	writeError(w, status, err)
}
