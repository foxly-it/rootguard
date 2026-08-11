package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/foxly-it/rootguard-core/internal/backupexport"
	"github.com/foxly-it/rootguard-core/internal/updater"
)

func TestBackupExportHandlerValidatesAndStreamsAgeArchive(t *testing.T) {
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "settings.json"), []byte("settings"), 0600); err != nil {
		t.Fatal(err)
	}
	exporter := backupexport.New(backupexport.Options{
		DataDir: t.TempDir(), LocalSources: []backupexport.Source{{ArchivePath: "rootguard/config", Path: source}},
		Run: func(context.Context, ...string) ([]byte, error) { return nil, nil },
	})
	manager := updater.NewManager(updater.Options{DataDir: t.TempDir()})
	handler := RegisterRoutes(Dependencies{Token: "secret", Updater: manager, BackupExporter: exporter})

	invalid := httptest.NewRequest(http.MethodPost, "/api/backups/export", bytes.NewBufferString(`{"passphrase":"short"}`))
	invalid.Header.Set("Authorization", "Bearer secret")
	invalidResponse := httptest.NewRecorder()
	handler.ServeHTTP(invalidResponse, invalid)
	if invalidResponse.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid passphrase rejection, got %d", invalidResponse.Code)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/backups/export", bytes.NewBufferString(`{"passphrase":"long-enough-passphrase"}`))
	request.Header.Set("Authorization", "Bearer secret")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.HasPrefix(response.Body.String(), "age-encryption.org/v1") {
		t.Fatalf("unexpected export response: code=%d body=%q", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "no-store" || !strings.Contains(response.Header().Get("Content-Disposition"), ".tar.gz.age") {
		t.Fatalf("missing protected download headers: %v", response.Header())
	}
}
