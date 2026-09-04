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

// newSelfUpdateTestManager builds a manager with both services this shared
// channel actually carries, mirroring main.go's real Services list -
// exercising the shared-manager shape these tests are meant to guard,
// not a single-service stand-in.
func newSelfUpdateTestManager(t *testing.T, run updater.CommandRunner) *updater.Manager {
	t.Helper()
	return updater.NewManager(updater.Options{
		DataDir: t.TempDir(), ComposeDir: t.TempDir(),
		Services: []updater.ServiceSpec{
			{Name: "updater", DisplayName: "RootGuard Updater", Container: "rootguard-updater"},
			{Name: "attestation-proxy", DisplayName: "Attestation Proxy", Container: "rootguard-attestation-proxy"},
		},
		Run: run,
	})
}

func selfUpdateInstallRequest(service string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/api/updater-updates/install/"+service, nil)
	request.SetPathValue("name", service)
	return request
}

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
	manager := newSelfUpdateTestManager(t, func(context.Context, ...string) ([]byte, error) {
		t.Fatal("docker should not run while the control plane is busy")
		return nil, nil
	})

	recorder := httptest.NewRecorder()
	selfUpdateInstallHandler(manager, controlPlane)(recorder, selfUpdateInstallRequest("updater"))

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
	manager := newSelfUpdateTestManager(t, func(context.Context, ...string) ([]byte, error) {
		return []byte("rootguard-updater:v1|sha256:old"), nil
	})

	recorder := httptest.NewRecorder()
	selfUpdateInstallHandler(manager, controlPlane)(recorder, selfUpdateInstallRequest("updater"))

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

// TestAttestationProxySelfUpdateInstallRefusesWhileControlPlaneBusy mirrors
// TestUpdaterSelfUpdateInstallRefusesWhileControlPlaneBusy for the
// attestation-proxy service on the *same* shared manager instance -
// proves selfUpdateInstallHandler's busy guard works identically for
// either service the shared channel carries (rootguard#481/#488: this
// manager used to be split into two separate instances with two separate
// mutexes, which raced against each other on the same compose project;
// sharing one manager closes that race, these tests guard the shared
// manager's own service-name dispatch instead of a second instance).
func TestAttestationProxySelfUpdateInstallRefusesWhileControlPlaneBusy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"state":"updating","message":"","services":[],"updated_at":"2026-01-01T00:00:00Z"}`))
	}))
	defer server.Close()

	controlPlane := controlplane.NewClient(server.URL, "token")
	manager := newSelfUpdateTestManager(t, func(context.Context, ...string) ([]byte, error) {
		t.Fatal("docker should not run while the control plane is busy")
		return nil, nil
	})

	recorder := httptest.NewRecorder()
	selfUpdateInstallHandler(manager, controlPlane)(recorder, selfUpdateInstallRequest("attestation-proxy"))

	if recorder.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict while the control plane is busy, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

// TestAttestationProxySelfUpdateInstallProceedsWhileControlPlaneIdle mirrors
// TestUpdaterSelfUpdateInstallProceedsWhileControlPlaneIdle for the same
// reason as the busy-guard test above.
func TestAttestationProxySelfUpdateInstallProceedsWhileControlPlaneIdle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"state":"idle","message":"","services":[],"updated_at":"2026-01-01T00:00:00Z"}`))
	}))
	defer server.Close()

	controlPlane := controlplane.NewClient(server.URL, "token")
	manager := newSelfUpdateTestManager(t, func(context.Context, ...string) ([]byte, error) {
		return []byte("rootguard-attestation-proxy:v1|sha256:old"), nil
	})

	recorder := httptest.NewRecorder()
	selfUpdateInstallHandler(manager, controlPlane)(recorder, selfUpdateInstallRequest("attestation-proxy"))

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202 Accepted while the control plane is idle, got %d: %s", recorder.Code, recorder.Body.String())
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && manager.Status().State == updater.StateUpdating {
		time.Sleep(10 * time.Millisecond)
	}
}

// TestSelfUpdateInstallRejectsUnknownService proves the shared manager's
// own StartUpdate validation still rejects a service name that isn't in
// its Services list - the shared channel doesn't silently accept an
// arbitrary path segment.
func TestSelfUpdateInstallRejectsUnknownService(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"state":"idle","message":"","services":[],"updated_at":"2026-01-01T00:00:00Z"}`))
	}))
	defer server.Close()

	controlPlane := controlplane.NewClient(server.URL, "token")
	manager := newSelfUpdateTestManager(t, func(context.Context, ...string) ([]byte, error) {
		t.Fatal("docker should not run for an unknown service")
		return nil, nil
	})

	recorder := httptest.NewRecorder()
	selfUpdateInstallHandler(manager, controlPlane)(recorder, selfUpdateInstallRequest("not-a-real-service"))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for an unknown service, got %d: %s", recorder.Code, recorder.Body.String())
	}
}
