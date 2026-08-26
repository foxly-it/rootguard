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

// TestDigestFromPullOutputParsesARealPullTranscript is the regression test
// for a real bug caught live in CI: digestQualify's RepoDigests lookup
// returned a *stale* digest (the previous release's, not the one just
// pulled) because RepoDigests belongs to the whole local image object, not
// to this specific pull. digestFromPullOutput reads the digest docker
// pull's own output reports instead, which can only ever be what was just
// pulled.
func TestDigestFromPullOutputParsesARealPullTranscript(t *testing.T) {
	output := []byte(`0.1.0-beta.10: Pulling from foxly-it/rootguard-core
Digest: sha256:d41fcc37dcb1bc76bf8d177ad164a90ee8e2070d1efe5e48cf52fa781781c54d
Status: Downloaded newer image for ghcr.io/foxly-it/rootguard-core:0.1.0-beta.10
`)
	got, ok := digestFromPullOutput("ghcr.io/foxly-it/rootguard-core:0.1.0-beta.10", output)
	if !ok {
		t.Fatal("expected a digest to be found")
	}
	want := "ghcr.io/foxly-it/rootguard-core@sha256:d41fcc37dcb1bc76bf8d177ad164a90ee8e2070d1efe5e48cf52fa781781c54d"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDigestFromPullOutputReturnsFalseWithoutADigestLine(t *testing.T) {
	if _, ok := digestFromPullOutput("repo:tag", []byte("some unexpected output\n")); ok {
		t.Fatal("expected no digest to be found")
	}
}

func TestDigestFromPullOutputReturnsFalseForATaglessImage(t *testing.T) {
	if _, ok := digestFromPullOutput("repo-with-no-colon", []byte("Digest: sha256:abc\n")); ok {
		t.Fatal("expected no digest to be found for an image reference with no tag separator")
	}
}

// TestIsOlderReleaseVersion is the regression test for the exact incident
// that motivated this check: beta.12's own release resolution had a bug
// (see rootguard-core's pickLatestReleaseImage) that could resolve
// "latest" to beta.9 - an older release than the one actually running.
// Nothing checked that before swapping.
func TestIsOlderReleaseVersion(t *testing.T) {
	tests := []struct {
		candidate, current string
		older, comparable  bool
	}{
		{"0.1.0-beta.9", "0.1.0-beta.12", true, true},
		{"0.1.0-beta.13", "0.1.0-beta.12", false, true},
		{"0.1.0-beta.12", "0.1.0-beta.12", false, true},
		{"0.1.0-alpha.20", "0.1.0-beta.1", true, true},
		{"0.1.0-beta.1", "0.1.0-alpha.20", false, true},
		{"dev", "0.1.0-beta.12", false, false},
		{"0.1.0-beta.9", "dev", false, false},
		{"", "", false, false},
	}
	for _, test := range tests {
		older, comparable := isOlderReleaseVersion(test.candidate, test.current)
		if older != test.older || comparable != test.comparable {
			t.Errorf("isOlderReleaseVersion(%q, %q) = (%v, %v), want (%v, %v)",
				test.candidate, test.current, older, comparable, test.older, test.comparable)
		}
	}
}

// TestUpdateRefusesADowngrade reproduces the beta.12->beta.9 incident
// end to end through update(): the resolved target image genuinely exists
// and differs from what's running (so the plain ID-comparison "changed"
// check alone would have let it through), but its
// org.opencontainers.image.version label identifies it as an older
// release. update() must refuse and report a failure instead of applying
// it.
func TestUpdateRefusesADowngrade(t *testing.T) {
	imageVersions := map[string]string{
		"sha256:core-old": "0.1.0-beta.12",
		"sha256:core-new": "0.1.0-beta.9", // the "candidate" - actually older
		"sha256:web-old":  "0.1.0-beta.12",
		"sha256:web-new":  "0.1.0-beta.9",
	}
	current := map[string]string{"rootguard-core": "sha256:core-old", "rootguard-webapp": "sha256:web-old"}
	candidates := map[string]string{"core:new": "sha256:core-new", "web:new": "sha256:web-new"}
	run := func(_ context.Context, arguments ...string) ([]byte, error) {
		switch {
		case arguments[0] == "inspect":
			container := arguments[len(arguments)-1]
			return []byte(container + ":image|" + current[container]), nil
		case len(arguments) >= 2 && arguments[0] == "image" && arguments[1] == "inspect" && strings.Contains(arguments[len(arguments)-2], "Labels"):
			image := arguments[len(arguments)-1]
			return []byte(imageVersions[image]), nil
		case arguments[0] == "image":
			return []byte(candidates[arguments[len(arguments)-1]]), nil
		default:
			t.Fatalf("unexpected docker command: %v", arguments)
			return nil, nil
		}
	}
	manager := newManager(t.TempDir(), "/compose.yaml", "rootguard", testSpecs(), run)
	manager.skipPull = true

	if _, err := manager.StartUpdate(nil); err != nil {
		t.Fatal(err)
	}
	waitForState(t, manager, stateFailed)
	result := manager.Status()
	if !strings.Contains(result.Message, "older than the currently running") {
		t.Fatalf("expected a downgrade-refusal message, got %q", result.Message)
	}
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
	if _, err := manager.StartCheck(nil); err != nil {
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

	if _, err := manager.StartUpdate(nil); err != nil {
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
