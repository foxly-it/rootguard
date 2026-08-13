package main

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestControlPlaneCheckComparesBothAllowlistedServices(t *testing.T) {
	current := map[string]string{"rootguard-core": "sha256:core-old", "rootguard-webapp": "sha256:web-old"}
	candidates := map[string]string{"core:new": "sha256:core-new", "web:new": "sha256:web-new"}
	run := func(_ context.Context, arguments ...string) ([]byte, error) {
		switch {
		case arguments[0] == "inspect":
			container := arguments[len(arguments)-1]
			return []byte(container + ":old|" + current[container]), nil
		case arguments[0] == "image":
			image := arguments[len(arguments)-1]
			return []byte(candidates[image]), nil
		case arguments[0] == "pull":
			return []byte("ok"), nil
		default:
			t.Fatalf("unexpected docker command: %v", arguments)
			return nil, nil
		}
	}
	manager := newManager(t.TempDir(), "/compose.yaml", "rootguard", testSpecs(), run)
	if _, err := manager.StartCheck(); err != nil {
		t.Fatal(err)
	}
	result := waitForState(t, manager, stateIdle)
	if len(result.Services) != 2 || !result.Services[0].UpdateAvailable || !result.Services[1].UpdateAvailable {
		t.Fatalf("expected updates for both allowlisted services: %#v", result.Services)
	}
}

func TestControlPlaneUpdateRollsBackBothImagesWhenHealthFails(t *testing.T) {
	var mu sync.Mutex
	current := map[string]string{"rootguard-core": "sha256:core-old", "rootguard-webapp": "sha256:web-old"}
	candidates := map[string]string{"core:new": "sha256:core-new", "web:new": "sha256:web-new"}
	composeCalls := 0
	run := func(_ context.Context, arguments ...string) ([]byte, error) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case arguments[0] == "inspect":
			container := arguments[len(arguments)-1]
			return []byte(container + ":image|" + current[container]), nil
		case arguments[0] == "image":
			return []byte(candidates[arguments[len(arguments)-1]]), nil
		case arguments[0] == "compose":
			composeCalls++
			if composeCalls == 1 {
				current["rootguard-core"] = candidates["core:new"]
				current["rootguard-webapp"] = candidates["web:new"]
			} else {
				current["rootguard-core"] = "sha256:core-old"
				current["rootguard-webapp"] = "sha256:web-old"
			}
			return []byte("ok"), nil
		default:
			t.Fatalf("unexpected docker command: %v", arguments)
			return nil, nil
		}
	}
	manager := newManager(t.TempDir(), "/compose.yaml", "rootguard", testSpecs(), run)
	manager.skipPull = true
	manager.verifyAttempts = 1
	manager.httpClient = &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		mu.Lock()
		defer mu.Unlock()
		code := http.StatusOK
		if current["rootguard-core"] == candidates["core:new"] {
			code = http.StatusServiceUnavailable
		}
		return &http.Response{StatusCode: code, Status: http.StatusText(code), Body: io.NopCloser(strings.NewReader(""))}, nil
	})}

	if _, err := manager.StartUpdate(); err != nil {
		t.Fatal(err)
	}
	result := waitForState(t, manager, stateFailed)
	if composeCalls != 2 {
		t.Fatalf("expected update and rollback compose calls, got %d", composeCalls)
	}
	if !strings.Contains(result.Message, "rolled back safely") {
		t.Fatalf("expected safe rollback status, got %q", result.Message)
	}
}

func TestControlPlaneCleanupNeverUsesGlobalPrune(t *testing.T) {
	var commands []string
	run := func(_ context.Context, arguments ...string) ([]byte, error) {
		command := strings.Join(arguments, " ")
		commands = append(commands, command)
		switch command {
		case "ps -a --filter ancestor=sha256:core-old --format {{.ID}}":
			return nil, nil
		case "image rm sha256:core-old":
			return []byte("removed"), nil
		case "volume ls --quiet --filter label=io.rootguard.cleanup=true":
			return nil, nil
		default:
			return nil, nil
		}
	}
	manager := newManager(t.TempDir(), "/compose.yaml", "rootguard", testSpecs(), run)
	manager.status.History = []historyEntry{
		{Outcome: "success", FromIDs: map[string]string{"core": "sha256:core-previous"}, ToIDs: map[string]string{"core": "sha256:core-current"}},
		{Outcome: "success", FromIDs: map[string]string{"core": "sha256:core-old"}, ToIDs: map[string]string{"core": "sha256:core-previous"}},
	}

	result := manager.cleanupAfterSuccess(context.Background())
	if strings.Join(result.RemovedImages, ",") != "sha256:core-old" {
		t.Fatalf("unexpected cleanup result: %#v", result)
	}
	all := strings.Join(commands, "\n")
	if strings.Contains(all, "sha256:core-previous") || strings.Contains(all, "sha256:core-current") ||
		strings.Contains(all, "prune") {
		t.Fatalf("cleanup touched a protected resource:\n%s", all)
	}
}

func TestInterruptedControlPlaneUpdateGetsRecoverableDiagnosticOnRestart(t *testing.T) {
	dataDir := t.TempDir()
	data := `{"state":"updating","message":"in progress","services":[],"updated_at":"2026-07-28T00:00:00Z"}`
	if err := os.WriteFile(filepath.Join(dataDir, "status.json"), []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	manager := newManager(dataDir, "/compose.yaml", "rootguard", testSpecs(), nil)
	status := manager.Status()
	if status.State != stateFailed || !strings.Contains(status.Message, "unterbrochen") {
		t.Fatalf("expected an interrupted-update diagnostic on restart, got %#v", status)
	}
}

func testSpecs() []serviceSpec {
	return []serviceSpec{
		{Name: "core", DisplayName: "Core", Container: "rootguard-core", TargetImage: "core:new", HealthURL: "http://core/health"},
		{Name: "webapp", DisplayName: "WebApp", Container: "rootguard-webapp", TargetImage: "web:new", HealthURL: "http://webapp/health"},
	}
}

func waitForState(t *testing.T, manager *manager, expected string) status {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		current := manager.Status()
		if current.State == expected {
			return current
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("manager did not reach state %s: %#v", expected, manager.Status())
	return status{}
}
