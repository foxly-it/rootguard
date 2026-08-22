package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/foxly-it/rootguard-core/internal/controlplane"
	"github.com/foxly-it/rootguard-core/internal/updater"
)

// TestUpdaterSelfUpdateInstallRefusesWhileControlPlaneBusy guards against
// the updater's own container being replaced while it's still mid a
// core/webapp check or update - the container swap would otherwise abort
// that in-flight operation.
func TestUpdaterSelfUpdateInstallRefusesWhileControlPlaneBusy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"state":"updating","message":"","services":[],"updated_at":"2026-01-01T00:00:00Z"}`))
	}))
	defer server.Close()

	controlPlane := controlplane.NewClient(server.URL, "token")
	manager := updater.NewManager(updater.Options{
		DataDir: t.TempDir(), ComposeDir: t.TempDir(),
		Services: []updater.ServiceSpec{{Name: "updater", DisplayName: "RootGuard Updater", Container: "rootguard-updater"}},
		Run: func(context.Context, ...string) ([]byte, error) {
			t.Fatal("docker should not run while the control plane is busy")
			return nil, nil
		},
	})

	request := httptest.NewRequest(http.MethodPost, "/api/updater-updates/install", nil)
	recorder := httptest.NewRecorder()
	updaterSelfUpdateInstallHandler(manager, controlPlane)(recorder, request)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict while the control plane is busy, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

// TestUpdaterSelfUpdateInstallProceedsWhileControlPlaneIdle is the inverse:
// a real update attempt must actually start once the control plane is idle,
// proving the guard above isn't just refusing everything unconditionally.
func TestUpdaterSelfUpdateInstallProceedsWhileControlPlaneIdle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"state":"idle","message":"","services":[],"updated_at":"2026-01-01T00:00:00Z"}`))
	}))
	defer server.Close()

	controlPlane := controlplane.NewClient(server.URL, "token")
	manager := updater.NewManager(updater.Options{
		DataDir: t.TempDir(), ComposeDir: t.TempDir(),
		Services: []updater.ServiceSpec{{Name: "updater", DisplayName: "RootGuard Updater", Container: "rootguard-updater"}},
		Run: func(context.Context, ...string) ([]byte, error) {
			return []byte("rootguard-updater:v1|sha256:old"), nil
		},
	})

	request := httptest.NewRequest(http.MethodPost, "/api/updater-updates/install", nil)
	recorder := httptest.NewRecorder()
	updaterSelfUpdateInstallHandler(manager, controlPlane)(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202 Accepted while the control plane is idle, got %d: %s", recorder.Code, recorder.Body.String())
	}

	// StartUpdate launches a background goroutine; wait for it to settle
	// before the test's t.TempDir() cleanup races its file writes.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && manager.Status().State == updater.StateUpdating {
		time.Sleep(10 * time.Millisecond)
	}
}
