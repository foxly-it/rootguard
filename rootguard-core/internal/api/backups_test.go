package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/foxly-it/rootguard-core/internal/updater"
)

func TestPutBackupSettingsHandlerValidatesAndPersistsRetention(t *testing.T) {
	manager := updater.NewManager(updater.Options{DataDir: t.TempDir()})
	handler := putBackupSettingsHandler(manager)

	invalid := httptest.NewRecorder()
	handler.ServeHTTP(invalid, httptest.NewRequest(http.MethodPut, "/api/backups/settings", strings.NewReader(`{"retention_per_service":1}`)))
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d: %s", invalid.Code, invalid.Body.String())
	}

	valid := httptest.NewRecorder()
	handler.ServeHTTP(valid, httptest.NewRequest(http.MethodPut, "/api/backups/settings", strings.NewReader(`{"retention_per_service":2}`)))
	if valid.Code != http.StatusOK || !strings.Contains(valid.Body.String(), `"retention_per_service":2`) {
		t.Fatalf("unexpected response %d: %s", valid.Code, valid.Body.String())
	}
	status, err := manager.BackupStatus()
	if err != nil || status.Settings.RetentionPerService != 2 {
		t.Fatalf("retention was not applied: status=%+v err=%v", status, err)
	}
}
