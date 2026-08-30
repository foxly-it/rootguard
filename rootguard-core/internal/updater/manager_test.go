package updater

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// noopAttestationVerifier lets tests that aren't specifically about the
// attestation gate itself exercise a real update() flow without needing
// actual cosign infrastructure or a ghcr.io-shaped image reference -
// stack.RequireAttestation (the real default) fails closed on a
// non-matching image for any service with a real signing policy, "unbound"
// included, which every fixture target image here is.
func noopAttestationVerifier(context.Context, string, string) error { return nil }

func TestCheckComparesRunningAndPulledImageIDs(t *testing.T) {
	manager := NewManager(Options{
		DataDir: t.TempDir(), ComposeDir: t.TempDir(),
		Services: []ServiceSpec{{
			Name: "adguard", DisplayName: "AdGuard Home", Container: "rootguard-adguard",
			TargetImage: "adguard/adguardhome:latest",
		}},
		Run: func(_ context.Context, arguments ...string) ([]byte, error) {
			switch arguments[0] {
			case "inspect":
				return []byte("adguard/adguardhome:v1|sha256:old"), nil
			case "pull":
				return []byte("pulled"), nil
			case "image":
				return []byte("sha256:new"), nil
			default:
				return nil, errors.New("unexpected command")
			}
		},
	})

	if _, err := manager.StartCheck(); err != nil {
		t.Fatal(err)
	}
	waitForIdle(t, manager)
	service := manager.Status().Services[0]
	if !service.UpdateAvailable || service.CurrentID != "sha256:old" || service.CandidateID != "sha256:new" {
		t.Fatalf("unexpected update result: %#v", service)
	}
}

func TestUpdateBacksUpAndVerifiesBeforeSuccess(t *testing.T) {
	dataDir := t.TempDir()
	composeDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(composeDir, "compose.yaml"), []byte("services: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var commands []string
	manager := NewManager(Options{
		DataDir: dataDir, ComposeDir: composeDir,
		Services: []ServiceSpec{{
			Name: "unbound", DisplayName: "Unbound", Container: "rootguard-unbound",
			TargetImage: "rootguard-unbound:latest", BackupPaths: []string{"/etc/unbound/unbound.d"},
		}},
		AttestationVerifier: noopAttestationVerifier,
		Run: func(_ context.Context, arguments ...string) ([]byte, error) {
			mu.Lock()
			commands = append(commands, strings.Join(arguments, " "))
			mu.Unlock()
			switch arguments[0] {
			case "inspect":
				return []byte("rootguard-unbound:v1|sha256:old"), nil
			case "image":
				return []byte("sha256:new"), nil
			default:
				return []byte("ok"), nil
			}
		},
		Verify: func(_ context.Context, service string) error {
			if service != "unbound" {
				t.Fatalf("unexpected service %q", service)
			}
			return nil
		},
	})

	if _, err := manager.StartUpdate("unbound"); err != nil {
		t.Fatal(err)
	}
	waitForIdle(t, manager)
	result := manager.Status()
	if result.State != StateIdle || result.Services[0].CurrentID != "sha256:new" {
		t.Fatalf("unexpected final status: %#v", result)
	}
	mu.Lock()
	all := strings.Join(commands, "\n")
	mu.Unlock()
	for _, expected := range []string{"cp rootguard-unbound:/etc/unbound/unbound.d", "pull rootguard-unbound:latest", "compose --project-name rootguard-dns"} {
		if !strings.Contains(all, expected) {
			t.Fatalf("missing command %q in:\n%s", expected, all)
		}
	}
}

// TestUpdateRefusesActivationWhenAttestationFails is the regression test
// for a real gap found in review: attestation was only ever checked for
// display (the stack status API), never enforced before an update actually
// activated the new image - selectImage/compose up ran unconditionally as
// soon as the health check passed. Proves the gate has teeth: a failing
// verifier must stop the update before any "compose" (activation) command
// is ever issued, not just after the fact.
func TestUpdateRefusesActivationWhenAttestationFails(t *testing.T) {
	dataDir := t.TempDir()
	composeDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(composeDir, "compose.yaml"), []byte("services: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var commands []string
	manager := NewManager(Options{
		DataDir: dataDir, ComposeDir: composeDir,
		Services: []ServiceSpec{{
			Name: "unbound", DisplayName: "Unbound", Container: "rootguard-unbound",
			TargetImage: "rootguard-unbound:latest", BackupPaths: []string{"/etc/unbound/unbound.d"},
		}},
		AttestationVerifier: func(context.Context, string, string) error {
			return errors.New("no matching signatures")
		},
		Run: func(_ context.Context, arguments ...string) ([]byte, error) {
			mu.Lock()
			commands = append(commands, strings.Join(arguments, " "))
			mu.Unlock()
			switch arguments[0] {
			case "inspect":
				return []byte("rootguard-unbound:v1|sha256:old"), nil
			case "image":
				return []byte("sha256:new"), nil
			default:
				return []byte("ok"), nil
			}
		},
		Verify: func(context.Context, string) error {
			t.Fatal("health check must not run when attestation was refused")
			return nil
		},
	})

	if _, err := manager.StartUpdate("unbound"); err != nil {
		t.Fatal(err)
	}
	waitForIdle(t, manager)
	result := manager.Status()
	if result.State != StateFailed || !strings.Contains(result.Services[0].Error, "no matching signatures") {
		t.Fatalf("expected a failed update citing the attestation error, got %#v", result)
	}
	mu.Lock()
	all := strings.Join(commands, "\n")
	mu.Unlock()
	if strings.Contains(all, "compose") {
		t.Fatalf("activation command ran despite refused attestation:\n%s", all)
	}
}

func TestUpdateMigratesExplicitVolumeOwnershipWithRestrictedHelper(t *testing.T) {
	dataDir := t.TempDir()
	composeDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(composeDir, "compose.yaml"), []byte("services: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var commands []string
	manager := NewManager(Options{
		DataDir: dataDir, ComposeDir: composeDir,
		Services: []ServiceSpec{{
			Name: "unbound", DisplayName: "Unbound", Container: "rootguard-unbound",
			TargetImage: "rootguard-unbound:latest",
			OwnershipMigrations: []VolumeOwnershipMigration{{
				Volume: "rootguard-unbound-state", Path: "/var/lib/unbound", UID: 100, GID: 101,
			}},
		}},
		AttestationVerifier: noopAttestationVerifier,
		Run: func(_ context.Context, arguments ...string) ([]byte, error) {
			command := strings.Join(arguments, " ")
			commands = append(commands, command)
			switch arguments[0] {
			case "inspect":
				return []byte("rootguard-unbound:v1|sha256:old"), nil
			case "image":
				return []byte("sha256:new"), nil
			case "run":
				if strings.Contains(command, "--entrypoint /usr/bin/stat") {
					return []byte("996:996\n"), nil
				}
				return []byte("changed"), nil
			default:
				return []byte("ok"), nil
			}
		},
	})

	if _, err := manager.StartUpdate("unbound"); err != nil {
		t.Fatal(err)
	}
	waitForIdle(t, manager)
	if manager.Status().State != StateIdle {
		t.Fatalf("expected successful update, got %#v", manager.Status())
	}
	all := strings.Join(commands, "\n")
	for _, expected := range []string{
		"run --rm --network none --user 0:0 --read-only --cap-drop ALL --volume rootguard-unbound-state:/var/lib/unbound:ro --entrypoint /usr/bin/stat sha256:old --format=%u:%g /var/lib/unbound",
		"run --rm --network none --user 0:0 --read-only --cap-drop ALL --cap-add CHOWN --security-opt no-new-privileges:true --volume rootguard-unbound-state:/var/lib/unbound --entrypoint /usr/bin/chown sha256:new --recursive 100:101 /var/lib/unbound",
	} {
		if !strings.Contains(all, expected) {
			t.Fatalf("missing restricted ownership command %q in:\n%s", expected, all)
		}
	}
}

func TestFailedUpdateRestoresPreviousVolumeOwnershipBeforeRollback(t *testing.T) {
	dataDir := t.TempDir()
	composeDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(composeDir, "compose.yaml"), []byte("services: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var commands []string
	verifyCalls := 0
	manager := NewManager(Options{
		DataDir: dataDir, ComposeDir: composeDir,
		VerifyAttempts: 1, RetryDelay: time.Millisecond,
		Services: []ServiceSpec{{
			Name: "unbound", DisplayName: "Unbound", Container: "rootguard-unbound",
			TargetImage: "rootguard-unbound:latest",
			OwnershipMigrations: []VolumeOwnershipMigration{{
				Volume: "rootguard-unbound-state", Path: "/var/lib/unbound", UID: 100, GID: 101,
			}},
		}},
		AttestationVerifier: noopAttestationVerifier,
		Run: func(_ context.Context, arguments ...string) ([]byte, error) {
			command := strings.Join(arguments, " ")
			commands = append(commands, command)
			switch arguments[0] {
			case "inspect":
				return []byte("rootguard-unbound:v1|sha256:old"), nil
			case "image":
				return []byte("sha256:new"), nil
			case "run":
				if strings.Contains(command, "--entrypoint /usr/bin/stat") {
					return []byte("996:996\n"), nil
				}
				return []byte("changed"), nil
			default:
				return []byte("ok"), nil
			}
		},
		Verify: func(context.Context, string) error {
			verifyCalls++
			if verifyCalls == 1 {
				return errors.New("candidate unhealthy")
			}
			return nil
		},
	})

	if _, err := manager.StartUpdate("unbound"); err != nil {
		t.Fatal(err)
	}
	waitForIdle(t, manager)
	if manager.Status().State != StateFailed {
		t.Fatalf("expected safe rollback, got %#v", manager.Status())
	}
	all := strings.Join(commands, "\n")
	targetChange := strings.Index(all, "--entrypoint /usr/bin/chown sha256:new --recursive 100:101")
	rollbackChange := strings.Index(all, "--entrypoint /usr/bin/chown sha256:old --recursive 996:996")
	rollbackCompose := strings.LastIndex(all, "compose --project-name rootguard-dns")
	if targetChange < 0 || rollbackChange <= targetChange || rollbackCompose <= rollbackChange {
		t.Fatalf("expected ownership rollback before old image compose-up:\n%s", all)
	}
}

func TestFailedRollbackRefusesTamperedBackupInsteadOfRestoringIt(t *testing.T) {
	dataDir := t.TempDir()
	composeDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(composeDir, "compose.yaml"), []byte("services: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	containerRoot := t.TempDir()
	confDir := filepath.Join(containerRoot, "opt", "adguardhome", "conf")
	if err := os.MkdirAll(confDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confDir, "AdGuardHome.yaml"), []byte("original configuration"), 0600); err != nil {
		t.Fatal(err)
	}

	var backupDir string
	var restoreInvoked bool
	manager := NewManager(Options{
		DataDir: dataDir, ComposeDir: composeDir,
		VerifyAttempts: 1, RetryDelay: time.Millisecond,
		Services: []ServiceSpec{{
			Name: "adguard", DisplayName: "AdGuard Home", Container: "rootguard-adguard",
			TargetImage: "adguard:latest", BackupPaths: []string{"/opt/adguardhome/conf"},
		}},
		Run: func(_ context.Context, arguments ...string) ([]byte, error) {
			switch arguments[0] {
			case "inspect":
				return []byte("adguard:v1|sha256:old"), nil
			case "cp":
				if !strings.HasPrefix(arguments[1], "rootguard-adguard:") {
					restoreInvoked = true
					return []byte("ok"), nil
				}
				source := strings.TrimPrefix(arguments[1], "rootguard-adguard:")
				backupDir = arguments[2]
				if err := copyDirForTest(filepath.Join(containerRoot, source), filepath.Join(backupDir, filepath.Base(source))); err != nil {
					return nil, err
				}
				return []byte("ok"), nil
			case "pull":
				// Tamper with the backup only after its manifest was already
				// written, simulating corruption discovered too late for the
				// manifest itself to reflect it.
				tampered := filepath.Join(backupDir, "conf", "AdGuardHome.yaml")
				if err := os.WriteFile(tampered, []byte("tampered"), 0600); err != nil {
					return nil, err
				}
				return []byte("ok"), nil
			case "image":
				return []byte("sha256:new"), nil
			default:
				return []byte("ok"), nil
			}
		},
		Verify: func(context.Context, string) error {
			return errors.New("candidate unhealthy")
		},
	})

	if _, err := manager.StartUpdate("adguard"); err != nil {
		t.Fatal(err)
	}
	waitForIdle(t, manager)
	result := manager.Status()
	if result.State != StateFailed || !strings.Contains(result.Message, "verify backup integrity") {
		t.Fatalf("expected a refused rollback due to backup integrity, got %#v", result)
	}
	if restoreInvoked {
		t.Fatal("a tampered backup must never be restored into the container")
	}
	if content, err := os.ReadFile(filepath.Join(confDir, "AdGuardHome.yaml")); err != nil || string(content) != "original configuration" {
		t.Fatalf("fake container content changed despite the refused restore: %q, err=%v", content, err)
	}
}

func copyDirForTest(src, dst string) error {
	return filepath.WalkDir(src, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0700)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0600)
	})
}

func TestInterruptedUpdateGetsRecoverableDiagnosticOnRestart(t *testing.T) {
	dataDir := t.TempDir()
	data := `{"state":"updating","active_service":"unbound","message":"in progress","services":[],"updated_at":"2026-07-28T00:00:00Z"}`
	if err := os.WriteFile(filepath.Join(dataDir, "status.json"), []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(Options{DataDir: dataDir})
	status := manager.Status()
	if status.State != StateFailed || !strings.Contains(status.Message, "unterbrochen") {
		t.Fatalf("expected an interrupted-update diagnostic on restart, got %#v", status)
	}
}

func TestUnknownServiceIsRejected(t *testing.T) {
	manager := NewManager(Options{DataDir: t.TempDir()})
	if _, err := manager.StartUpdate("webapp"); !errors.Is(err, ErrUnknownService) {
		t.Fatalf("expected unknown service error, got %v", err)
	}
}

func TestFailedHealthCheckRestoresPreviousImageAndBackup(t *testing.T) {
	dataDir := t.TempDir()
	composeDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(composeDir, "compose.yaml"), []byte("services: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	verifyCalls := 0
	manager := NewManager(Options{
		DataDir: dataDir, ComposeDir: composeDir,
		VerifyAttempts: 1, RetryDelay: time.Millisecond,
		Services: []ServiceSpec{{
			Name: "adguard", DisplayName: "AdGuard Home", Container: "rootguard-adguard",
			TargetImage: "adguard:latest", BackupPaths: []string{"/opt/adguardhome/conf"},
		}},
		Run: func(_ context.Context, arguments ...string) ([]byte, error) {
			switch arguments[0] {
			case "inspect":
				return []byte("adguard:v1|sha256:old"), nil
			case "image":
				return []byte("sha256:new"), nil
			default:
				return []byte("ok"), nil
			}
		},
		Verify: func(context.Context, string) error {
			verifyCalls++
			if verifyCalls == 1 {
				return errors.New("candidate unhealthy")
			}
			return nil
		},
	})

	if _, err := manager.StartUpdate("adguard"); err != nil {
		t.Fatal(err)
	}
	waitForIdle(t, manager)
	result := manager.Status()
	if result.State != StateFailed || !strings.Contains(result.Message, "rolled back safely") {
		t.Fatalf("expected safe rollback status, got %#v", result)
	}
	override, err := os.ReadFile(filepath.Join(dataDir, "updates.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(override), `image: "sha256:old"`) {
		t.Fatalf("expected previous image to stay pinned after rollback:\n%s", override)
	}
}

func TestCleanupKeepsTwoImagesAndOnlyRemovesLabeledUnusedVolumes(t *testing.T) {
	var commands []string
	manager := NewManager(Options{
		DataDir:  t.TempDir(),
		Services: []ServiceSpec{{Name: "unbound", TargetImage: "unbound:latest"}},
		Run: func(_ context.Context, arguments ...string) ([]byte, error) {
			command := strings.Join(arguments, " ")
			commands = append(commands, command)
			switch command {
			case "ps -a --filter ancestor=sha256:old --format {{.ID}}":
				return nil, nil
			case "system df -v --format {{json .Images}}":
				return []byte(`[{"ID":"sha256:old","UniqueSize":"12MB"}]`), nil
			case "image rm sha256:old":
				return []byte("removed"), nil
			case "volume ls --quiet --filter label=io.rootguard.cleanup=true":
				return []byte("rootguard-transient\n"), nil
			case "system df -v --format {{json .Volumes}}":
				return []byte(`[{"Name":"rootguard-transient","Size":"1.5MB"}]`), nil
			case "ps -a --filter volume=rootguard-transient --format {{.ID}}":
				return nil, nil
			case "volume rm rootguard-transient":
				return []byte("removed"), nil
			default:
				return nil, errors.New("unexpected command: " + command)
			}
		},
	})
	manager.status.History = []HistoryEntry{
		{Service: "unbound", Outcome: "success", FromID: "sha256:previous", ToID: "sha256:current"},
		{Service: "unbound", Outcome: "success", FromID: "sha256:old", ToID: "sha256:previous"},
	}

	result := manager.cleanupAfterSuccess(context.Background(), "unbound")
	if strings.Join(result.RemovedImages, ",") != "sha256:old" ||
		strings.Join(result.RemovedVolumes, ",") != "rootguard-transient" {
		t.Fatalf("unexpected cleanup result: %#v", result)
	}
	all := strings.Join(commands, "\n")
	if strings.Contains(all, "sha256:previous") || strings.Contains(all, "sha256:current") ||
		strings.Contains(all, "prune") {
		t.Fatalf("cleanup touched a protected resource:\n%s", all)
	}
}

func TestRunExclusiveBlocksConcurrentUpdaterOperations(t *testing.T) {
	manager := NewManager(Options{DataDir: t.TempDir()})
	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- manager.RunExclusive("export", func() error {
			close(entered)
			<-release
			return nil
		})
	}()
	<-entered
	if _, err := manager.StartCheck(); !errors.Is(err, ErrBusy) {
		t.Fatalf("expected updater operation to be blocked, got %v", err)
	}
	close(release)
	if err := <-done; err != nil || manager.Status().State != StateIdle {
		t.Fatalf("exclusive operation did not finish cleanly: err=%v status=%+v", err, manager.Status())
	}
}

func waitForIdle(t *testing.T, manager *Manager) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		state := manager.Status().State
		if state == StateIdle || state == StateFailed {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("operation did not finish")
}

// TestPersistLockedReportsFailureViaOnPersistError is the regression
// test for a review finding: nearly every call site of persistLocked
// discards its returned error entirely ("_ = m.persistLocked()"), so a
// full disk or permissions problem could leave Status silently out of
// date with no visibility anywhere. persistLocked now reports any
// failure through the injectable OnPersistError hook before returning
// it, regardless of what the caller does with the return value.
func TestPersistLockedReportsFailureViaOnPersistError(t *testing.T) {
	// A regular file where DataDir expects a directory - os.MkdirAll
	// reliably fails against this on every OS.
	blocker := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blocker, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	dataDir := filepath.Join(blocker, "updates")

	var reported error
	manager := NewManager(Options{
		DataDir:        dataDir,
		OnPersistError: func(err error) { reported = err },
	})

	if err := manager.persist(); err == nil {
		t.Fatal("expected persist to fail against a path blocked by a file")
	}
	if reported == nil {
		t.Fatal("expected OnPersistError to be called with the failure")
	}
}

// TestStatusSurfacesPersistFailureAndSelfHeals mirrors the installer
// package's own test of the same name - see its doc comment for the full
// rationale.
func TestStatusSurfacesPersistFailureAndSelfHeals(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blocker, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	dataDir := filepath.Join(blocker, "updates")

	manager := NewManager(Options{DataDir: dataDir})

	if err := manager.persist(); err == nil {
		t.Fatal("expected persist to fail against a path blocked by a file")
	}
	status := manager.Status()
	if status.PersistError == "" {
		t.Fatal("expected Status() to report a persist error")
	}
	if status.PersistErrorAt.IsZero() {
		t.Fatal("expected Status() to report when the persist error occurred")
	}

	manager.dataDir = t.TempDir()
	if err := manager.persist(); err != nil {
		t.Fatal(err)
	}
	status = manager.Status()
	if status.PersistError != "" {
		t.Fatalf("expected PersistError to clear after a successful persist, got %q", status.PersistError)
	}
	if !status.PersistErrorAt.IsZero() {
		t.Fatalf("expected PersistErrorAt to clear after a successful persist, got %v", status.PersistErrorAt)
	}
}

// TestPersistLockedStateJSONIsSingleFileAtomic is the regression test for
// a follow-up review finding: even after persistLocked started staging
// status.json/images.json/updates.yaml through one atomicfile.WriteFiles
// call (round 3's own fix), renaming several files can never be one
// atomic operation on POSIX - a rename failing (or the process dying)
// between two renames still left status.json and images.json in two
// different generations, the exact scenario the fix was supposed to
// close. This test's own predecessor (of the same name pattern)
// demonstrated that residual window deliberately.
//
// Closed for real now: m.status and m.selected are one canonical file,
// state.json, written with a single atomicfile.WriteJSON call - and a
// single file's write-temp-then-rename is unconditionally atomic, no
// residual window at all. Proven here by sabotaging updates.yaml (now a
// separate, purely derived, self-healing artifact - see
// TestUpdatesYAMLSelfHealsAfterAFailedPersist below) so its own rename
// fails, and confirming state.json still updates correctly regardless -
// a failure in the derived artifact must never block the canonical state
// from advancing.
//
// Exercises persistLocked directly, not through selectImage - found in a
// later review round that selectImage itself now reverts m.selected (and
// re-persists that reversion) on any persist failure, specifically so a
// failed *operation* never leaves the canonical state claiming an image
// selection that was never actually applied (see
// TestSelectImageRevertsSelectionOnPersistFailure below). That's a
// property of selectImage as a whole, orthogonal to what this test
// actually verifies about persistLocked's own single-call behavior -
// testing through selectImage here would just be asserting selectImage's
// revert logic a second time under a name that says something else.
func TestPersistLockedStateJSONIsSingleFileAtomic(t *testing.T) {
	dataDir := t.TempDir()
	manager := NewManager(Options{
		DataDir: dataDir,
		Services: []ServiceSpec{{
			Name: "core", DisplayName: "Core", TargetImage: "rootguard-core:latest",
		}},
	})

	manager.selected["core"] = "rootguard-core:gen1"
	if err := manager.persist(); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(dataDir, "state.json")
	overridePath := filepath.Join(dataDir, "updates.yaml")

	// Replace updates.yaml with a directory - os.Rename(tempFile,
	// overridePath) reliably fails against this on every OS. state.json
	// is first in persistLocked's WriteFiles call, so this leaves its own
	// rename already committed regardless of what happens to updates.yaml
	// next.
	if err := os.Remove(overridePath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(overridePath, 0700); err != nil {
		t.Fatal(err)
	}

	manager.selected["core"] = "rootguard-core:gen2"
	if err := manager.persist(); err == nil {
		t.Fatal("expected the persist to fail (updates.yaml's rename can't succeed against a directory)")
	}

	stateAfter, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(stateAfter), "rootguard-core:gen2") {
		t.Fatalf("expected state.json to still advance to gen2 despite updates.yaml's own failure, got:\n%s", stateAfter)
	}
}

// TestUpdatesYAMLSelfHealsAfterAFailedPersist is
// TestPersistLockedStateJSONIsSingleFileAtomic's other half: updates.yaml
// is a pure function of m.selected (part of state.json's own canonical
// content), not a second source of truth - so once whatever blocked its
// own write is gone, the very next successful persist must bring it back
// in sync with whatever the canonical state actually is at that point.
// Exercises persistLocked directly, not selectImage - see the sibling
// test's own comment on why.
func TestUpdatesYAMLSelfHealsAfterAFailedPersist(t *testing.T) {
	dataDir := t.TempDir()
	manager := NewManager(Options{
		DataDir: dataDir,
		Services: []ServiceSpec{{
			Name: "core", DisplayName: "Core", TargetImage: "rootguard-core:latest",
		}},
	})
	overridePath := filepath.Join(dataDir, "updates.yaml")

	manager.selected["core"] = "rootguard-core:gen1"
	if err := manager.persist(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(overridePath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(overridePath, 0700); err != nil {
		t.Fatal(err)
	}
	manager.selected["core"] = "rootguard-core:gen2"
	if err := manager.persist(); err == nil {
		t.Fatal("expected this persist to fail while updates.yaml is blocked")
	}

	// Whatever blocked updates.yaml is gone now - the very next persist
	// must self-heal it to match the current (gen2) canonical state.
	if err := os.Remove(overridePath); err != nil {
		t.Fatal(err)
	}
	if err := manager.persist(); err != nil {
		t.Fatal(err)
	}
	healed, err := os.ReadFile(overridePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(healed), "rootguard-core:gen2") {
		t.Fatalf("expected updates.yaml to self-heal to the gen2 image, got:\n%s", healed)
	}
}

// TestSelectImageRevertsSelectionOnPersistFailure is the regression test
// for a follow-up review finding: selectImage set m.selected to the new
// image *before* persisting, and a persist failure (e.g. updates.yaml's
// own write failing, as sabotaged below) still left that new image
// selected - both in memory and, whenever state.json's own write inside
// that same failed attempt happened to succeed anyway (see
// TestPersistLockedStateJSONIsSingleFileAtomic above - it does), on disk
// too. Every real caller (update()) treats a selectImage failure as the
// whole operation failing and rolls back everything else it already did
// (volume ownership migration, the container swap that never happens) -
// so the canonical selection quietly advancing past a change that was
// never actually applied was a real, if narrow, inconsistency: a later,
// unrelated successful persist would self-heal updates.yaml to an image
// this process itself had already finished rolling back away from.
func TestSelectImageRevertsSelectionOnPersistFailure(t *testing.T) {
	dataDir := t.TempDir()
	manager := NewManager(Options{
		DataDir: dataDir,
		Services: []ServiceSpec{{
			Name: "core", DisplayName: "Core", TargetImage: "rootguard-core:latest",
		}},
	})
	statePath := filepath.Join(dataDir, "state.json")
	overridePath := filepath.Join(dataDir, "updates.yaml")

	if err := manager.selectImage("core", "rootguard-core:gen1"); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(overridePath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(overridePath, 0700); err != nil {
		t.Fatal(err)
	}

	if err := manager.selectImage("core", "rootguard-core:gen2"); err == nil {
		t.Fatal("expected selectImage to fail while updates.yaml is blocked")
	}

	if got := manager.selected["core"]; got != "rootguard-core:gen1" {
		t.Fatalf("expected the in-memory selection to revert to gen1 after the failed selectImage, got %q", got)
	}
	stateAfter, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(stateAfter), "rootguard-core:gen2") {
		t.Fatalf("expected state.json to have reverted to gen1, not still claim gen2 was selected, got:\n%s", stateAfter)
	}
	if !strings.Contains(string(stateAfter), "rootguard-core:gen1") {
		t.Fatalf("expected state.json to still show the reverted gen1 selection, got:\n%s", stateAfter)
	}
}

// TestSelectImageRevertsToNoSelectionOnFirstPersistFailure is the sibling
// of TestSelectImageRevertsSelectionOnPersistFailure above for a service
// that had never been selected before - found in review, round 6: the
// revert used to unconditionally write m.selected[service] = previous,
// which for a never-selected service (previous is Go's zero value "" for
// a missing key) left behind an explicit service: "" map entry instead of
// no entry at all. overrideContentLocked's own TargetImage fallback
// treats both the same, so this never actually surfaced - but a reverted
// selectImage should leave the exact state it found, not a lookalike.
func TestSelectImageRevertsToNoSelectionOnFirstPersistFailure(t *testing.T) {
	dataDir := t.TempDir()
	manager := NewManager(Options{
		DataDir: dataDir,
		Services: []ServiceSpec{{
			Name: "core", DisplayName: "Core", TargetImage: "rootguard-core:latest",
		}},
	})
	overridePath := filepath.Join(dataDir, "updates.yaml")

	if err := os.Mkdir(overridePath, 0700); err != nil {
		t.Fatal(err)
	}

	if err := manager.selectImage("core", "rootguard-core:gen1"); err == nil {
		t.Fatal("expected selectImage to fail while updates.yaml is blocked")
	}

	if _, existed := manager.selected["core"]; existed {
		t.Fatalf("expected no map entry for a never-selected service after the failed selectImage, got %q", manager.selected["core"])
	}
}

// TestLoadMigratesLegacyStatusAndImagesJSON is the regression test for
// the migration path load() needs now that status.json/images.json were
// consolidated into state.json: every real installation up to and
// including 1.0.0-rc.1 has data directories in the old split-file shape
// on disk, and must not lose their update history/selected-image state
// the first time they run a Core build with this change.
func TestLoadMigratesLegacyStatusAndImagesJSON(t *testing.T) {
	dataDir := t.TempDir()
	legacyStatus := `{"state":"idle","message":"legacy state","services":[],"updated_at":"2026-08-29T00:00:00Z"}`
	legacyImages := `{"core":"rootguard-core:legacy-pin"}`
	if err := os.WriteFile(filepath.Join(dataDir, "status.json"), []byte(legacyStatus), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "images.json"), []byte(legacyImages), 0600); err != nil {
		t.Fatal(err)
	}

	manager := NewManager(Options{DataDir: dataDir})

	status := manager.Status()
	if status.Message != "legacy state" {
		t.Fatalf("expected the legacy status.json content to be loaded, got %+v", status)
	}
	if manager.selected["core"] != "rootguard-core:legacy-pin" {
		t.Fatalf("expected the legacy images.json content to be loaded, got %+v", manager.selected)
	}

	// Migrated to state.json immediately (load() re-persists once after a
	// successful legacy read), not deferred until the next unrelated
	// status change.
	stateData, err := os.ReadFile(filepath.Join(dataDir, "state.json"))
	if err != nil {
		t.Fatalf("expected state.json to exist after migration, got: %v", err)
	}
	if !strings.Contains(string(stateData), "legacy state") || !strings.Contains(string(stateData), "rootguard-core:legacy-pin") {
		t.Fatalf("expected state.json to contain the migrated legacy content, got:\n%s", stateData)
	}

	// A second Manager pointed at the same, now-migrated directory must
	// load from state.json directly - the legacy files are stale
	// leftovers from here on, never read again.
	reloaded := NewManager(Options{DataDir: dataDir})
	if reloaded.Status().Message != "legacy state" {
		t.Fatalf("expected the reloaded manager to read the migrated state.json, got %+v", reloaded.Status())
	}
}
