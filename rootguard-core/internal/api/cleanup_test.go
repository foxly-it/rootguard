package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/foxly-it/rootguard-core/internal/updater"
)

func TestCleanupPreviewAndExecutionHandlers(t *testing.T) {
	removed := false
	manager := updater.NewManager(updater.Options{
		DataDir: t.TempDir(),
		Run: func(_ context.Context, arguments ...string) ([]byte, error) {
			switch strings.Join(arguments, " ") {
			case "volume ls --quiet --filter label=io.rootguard.cleanup=true":
				return []byte("transient"), nil
			case "ps -a --filter volume=transient --format {{.ID}}":
				return nil, nil
			case "system df -v --format {{json .Volumes}}":
				return []byte(`[{"Name":"transient","Size":"2kB"}]`), nil
			case "volume rm transient":
				removed = true
				return nil, nil
			default:
				return nil, nil
			}
		},
	})
	handler := RegisterRoutes(Dependencies{Token: "secret", Updater: manager})

	previewRequest := httptest.NewRequest(http.MethodGet, "/api/cleanup/preview", nil)
	previewRequest.Header.Set("Authorization", "Bearer secret")
	previewResponse := httptest.NewRecorder()
	handler.ServeHTTP(previewResponse, previewRequest)
	if previewResponse.Code != http.StatusOK {
		t.Fatalf("unexpected preview response: %d %s", previewResponse.Code, previewResponse.Body.String())
	}
	var preview updater.CleanupPreview
	if err := json.NewDecoder(previewResponse.Body).Decode(&preview); err != nil || len(preview.Resources) != 1 || preview.EstimatedBytes != 2000 {
		t.Fatalf("unexpected preview: %+v err=%v", preview, err)
	}

	runRequest := httptest.NewRequest(http.MethodPost, "/api/cleanup", bytes.NewReader(nil))
	runRequest.Header.Set("Authorization", "Bearer secret")
	runResponse := httptest.NewRecorder()
	handler.ServeHTTP(runResponse, runRequest)
	if runResponse.Code != http.StatusOK || !removed {
		t.Fatalf("unexpected cleanup response: %d %s removed=%v", runResponse.Code, runResponse.Body.String(), removed)
	}
}
