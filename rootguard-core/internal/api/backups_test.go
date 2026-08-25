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

// TestPutBackupSettingsHandlerRejectsTrailingData is a regression test for
// the shared decodeJSON helper (see routes.go) - this handler used to run
// its own bare decoder.Decode() with no check for a second JSON value
// appended after the first, unlike the AdGuard handlers that already got
// this fix. A code review flagged this as unified across every Core
// handler at once via the new helper, rather than case by case.
func TestPutBackupSettingsHandlerRejectsTrailingData(t *testing.T) {
	manager := updater.NewManager(updater.Options{DataDir: t.TempDir()})
	handler := putBackupSettingsHandler(manager)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPut, "/api/backups/settings", strings.NewReader(`{"retention_per_service":2}{"extra":true}`)))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected trailing JSON data to be rejected with 400, got %d: %s", response.Code, response.Body.String())
	}
}
