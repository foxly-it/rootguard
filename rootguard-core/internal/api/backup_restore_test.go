package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/foxly-it/rootguard-core/internal/backupexport"
	"github.com/foxly-it/rootguard-core/internal/backuprestore"
	"github.com/foxly-it/rootguard-core/internal/installer"
)

func TestBackupRestorePreviewHandlerValidatesArchiveAndCleanTarget(t *testing.T) {
	encrypted := validRestoreArchive(t)
	run := func(_ context.Context, arguments ...string) ([]byte, error) {
		if len(arguments) >= 2 && (arguments[0] == "container" || arguments[0] == "volume" || arguments[0] == "network") && arguments[1] == "inspect" {
			return []byte("no such object"), errors.New("not found")
		}
		return nil, nil
	}
	installation := installer.NewManager(installer.Options{DataDir: t.TempDir(), Run: run})
	restorer := backuprestore.New(backuprestore.Options{DataDir: t.TempDir(), Installer: installation, Run: run})
	handler := RegisterRoutes(Dependencies{Token: "secret", BackupRestorer: restorer})

	body, contentType := restoreMultipart(t, encrypted, "correct horse battery staple")
	request := httptest.NewRequest(http.MethodPost, "/api/backups/restore/preview", body)
	request.Header.Set("Authorization", "Bearer secret")
	request.Header.Set("Content-Type", contentType)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("unexpected response %d: %s", response.Code, response.Body.String())
	}
	var preview backuprestore.PreviewResult
	if err := json.Unmarshal(response.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if !preview.Preflight.Ready || preview.FileCount != 3 {
		t.Fatalf("unexpected preview: %+v", preview)
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("preview may be cached")
	}
}

func validRestoreArchive(t *testing.T) []byte {
	t.Helper()
	installation, credentials, config := t.TempDir(), t.TempDir(), t.TempDir()
	status := `{"state":"installed","config":{"dns_bind_address":"192.0.2.10","dns_port":53,"adguard_channel":"stable","blockpage_enabled":false}}`
	for path, content := range map[string]string{
		filepath.Join(installation, "status.json"):     status,
		filepath.Join(credentials, "credentials.json"): `{"username":"rootguard","password":"secret"}`,
		filepath.Join(config, "AdGuardHome.yaml"):      "schema_version: 29",
	} {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	exporter := backupexport.New(backupexport.Options{DataDir: t.TempDir(), LocalSources: []backupexport.Source{
		{ArchivePath: "rootguard/installation", Path: installation}, {ArchivePath: "rootguard/adguard", Path: credentials}, {ArchivePath: "services/adguard/config", Path: config},
	}})
	var encrypted bytes.Buffer
	if err := exporter.Export(context.Background(), "correct horse battery staple", &encrypted); err != nil {
		t.Fatal(err)
	}
	return encrypted.Bytes()
}

func restoreMultipart(t *testing.T, encrypted []byte, passphrase string) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("passphrase", passphrase); err != nil {
		t.Fatal(err)
	}
	archive, err := writer.CreateFormFile("archive", "backup.tar.gz.age")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := archive.Write(encrypted); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return &body, writer.FormDataContentType()
}
