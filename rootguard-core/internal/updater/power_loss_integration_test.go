//go:build integration

// Proves Manager survives a real process kill (simulating power loss or an
// OOM kill) between the discrete steps of an update, not just a torn file
// write - every persisted file already uses atomic temp+rename, which this
// test does not need to re-prove. Since Go has no per-goroutine kill, the
// only way to genuinely simulate a killed process is to run the operation in
// a real child OS process and SIGKILL it - the same "TestHelperProcess"
// technique Go's own os/exec tests use: the compiled test binary re-execs
// itself (os.Args[0]) with a distinct -test.run selecting
// TestUpdateHelperProcess, which no other invocation of this package's tests
// ever selects.
// Requires Docker. Run with:
// go test -tags integration ./internal/updater/... -run TestUpdatePowerLoss -v
package updater

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestUpdateHelperProcess is never run directly by the normal test suite (it
// self-skips unless ROOTGUARD_HELPER_PROCESS is set) - it is the child body
// launched by killHelperProcessAtCheckpoint below.
func TestUpdateHelperProcess(t *testing.T) {
	if os.Getenv("ROOTGUARD_HELPER_PROCESS") != "1" {
		t.Skip("only runs as a subprocess helper for the power-loss tests")
	}
	checkpoints := os.Getenv("ROOTGUARD_HELPER_CHECKPOINTS")
	container := os.Getenv("ROOTGUARD_HELPER_CONTAINER")
	touch := func(name string) {
		_ = os.WriteFile(filepath.Join(checkpoints, name), nil, 0600)
	}
	manager := NewManager(Options{
		DataDir: os.Getenv("ROOTGUARD_HELPER_DATA_DIR"), ComposeDir: os.Getenv("ROOTGUARD_HELPER_COMPOSE_DIR"),
		ComposeProject: fixtureComposeProject, VerifyAttempts: 30, RetryDelay: time.Second,
		Run: helperCommandRunner(touch),
		Verify: func(ctx context.Context, service string) error {
			touch("verify-started")
			return fixtureHealthVerify(ctx, service)
		},
		Services: []ServiceSpec{{
			Name: "fixture", Container: container,
			TargetImage: os.Getenv("ROOTGUARD_HELPER_TARGET_IMAGE"), BackupPaths: []string{"/data"},
		}},
	})
	if _, err := manager.StartUpdate("fixture"); err != nil {
		t.Fatal(err)
	}
	select {} // block until the parent SIGKILLs this process at the desired checkpoint
}

// helperCommandRunner mirrors realDockerSkippingPull (rollback_integration_test.go)
// but additionally marks the "backup-done" checkpoint right after the real
// backup docker-cp completes and its checksummed manifest is already
// written - the same moment Manager.update itself moves on to the pull
// step, immediately before the candidate image is ever swapped in.
func helperCommandRunner(touch func(name string)) CommandRunner {
	return func(ctx context.Context, arguments ...string) ([]byte, error) {
		if len(arguments) > 0 && arguments[0] == "pull" {
			touch("backup-done")
			return []byte("skipped: local fixture image"), nil
		}
		return runDocker(ctx, arguments...)
	}
}

// killHelperProcessAtCheckpoint launches the helper process, waits for it to
// signal the named checkpoint file, then SIGKILLs it - simulating the host
// losing power at exactly that point in the update. It always waits for the
// process to actually exit before returning, so the caller's own docker
// commands never race a still-dying child.
func killHelperProcessAtCheckpoint(t *testing.T, dataDir, composeDir, targetImage, checkpoint string) {
	t.Helper()
	checkpoints := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestUpdateHelperProcess$", "-test.v")
	cmd.Env = append(os.Environ(),
		"ROOTGUARD_HELPER_PROCESS=1",
		"ROOTGUARD_HELPER_DATA_DIR="+dataDir,
		"ROOTGUARD_HELPER_COMPOSE_DIR="+composeDir,
		"ROOTGUARD_HELPER_CONTAINER="+fixtureContainer,
		"ROOTGUARD_HELPER_TARGET_IMAGE="+targetImage,
		"ROOTGUARD_HELPER_CHECKPOINTS="+checkpoints,
	)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start helper process: %v", err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _, _ = cmd.Process.Wait() })

	deadline := time.Now().Add(30 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(checkpoints, checkpoint)); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("helper process never reached checkpoint %q", checkpoint)
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatalf("kill helper process: %v", err)
	}
	_, _ = cmd.Process.Wait()
}

// restartManager builds a fresh Manager against the same on-disk DataDir a
// killed process left behind - the equivalent of RootGuard Core restarting
// after a crash or reboot - configured to retry the update against
// targetImage, the way an operator would relaunch Core with its normal
// (possibly corrected) target image after an interruption.
func restartManager(t *testing.T, dataDir, composeDir, targetImage string) *Manager {
	t.Helper()
	return NewManager(Options{
		DataDir: dataDir, ComposeDir: composeDir, ComposeProject: fixtureComposeProject,
		VerifyAttempts: 15, RetryDelay: time.Second,
		Run: realDockerSkippingPull(nil), Verify: fixtureHealthVerify,
		Services: []ServiceSpec{{
			Name: "fixture", Container: fixtureContainer,
			TargetImage: targetImage, BackupPaths: []string{"/data"},
		}},
	})
}

func onlyBackupManifestDir(t *testing.T, dataDir string) string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dataDir, "backups", "*", "fixture"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("expected exactly one captured backup, found %v (err=%v)", matches, err)
	}
	return matches[0]
}

func TestUpdatePowerLossBetweenBackupAndImageSwapRecoversCleanly(t *testing.T) {
	buildFixtureImages(t)
	composeDir := writeFixtureCompose(t)
	startFixtureContainer(t, composeDir, fixtureOldImage)
	oldID := dockerImageID(t, fixtureOldImage)
	dataDir := t.TempDir()

	killHelperProcessAtCheckpoint(t, dataDir, composeDir, fixtureBadImage, "backup-done")

	// The kill landed before the candidate image was ever swapped in - the
	// container must be completely untouched.
	if got := dockerRunningImageID(t); got != oldID {
		t.Fatalf("container image changed despite being killed before any swap: got %s want %s", got, oldID)
	}
	// The snapshot captured just before the kill must still be intact and
	// trustworthy: atomic writes protect both the copied files and the
	// manifest describing them, even though the surrounding multi-step
	// operation was aborted immediately afterward.
	backupDir := onlyBackupManifestDir(t, dataDir)
	if err := verifyBackupManifest(backupDir, ServiceSpec{Name: "fixture", Container: fixtureContainer}); err != nil {
		t.Fatalf("backup captured before the kill did not survive intact: %v", err)
	}

	// A fresh Manager against the same on-disk state - the equivalent of
	// Core restarting after the crash - must report the interruption
	// instead of a stale in-progress status.
	manager := restartManager(t, dataDir, composeDir, fixtureRecoveredImage)
	if manager.Status().State != StateFailed || !strings.Contains(manager.Status().Message, "unterbrochen") {
		t.Fatalf("expected an interrupted-update diagnostic after restart, got %#v", manager.Status())
	}

	// The appliance must not be stuck: a retried update now succeeds.
	recoveredID := dockerImageID(t, fixtureRecoveredImage)
	if _, err := manager.StartUpdate("fixture"); err != nil {
		t.Fatal(err)
	}
	waitForIdleWithin(t, manager, 90*time.Second)
	if manager.Status().State != StateIdle {
		t.Fatalf("expected the retried update to succeed, got %#v", manager.Status())
	}
	if got := dockerRunningImageID(t); got != recoveredID {
		t.Fatalf("expected the container on the recovered image %s after retry, got %s", recoveredID, got)
	}
}

func TestUpdatePowerLossAfterImageSwapBeforeVerifyRecoversCleanly(t *testing.T) {
	buildFixtureImages(t)
	composeDir := writeFixtureCompose(t)
	startFixtureContainer(t, composeDir, fixtureOldImage)
	dataDir := t.TempDir()

	killHelperProcessAtCheckpoint(t, dataDir, composeDir, fixtureBadImage, "verify-started")

	// The candidate image was already swapped in when the kill landed, and
	// nothing ever got a chance to verify or roll it back - the container is
	// left running the unverified (here: unhealthy) candidate. That is the
	// correct, realistic outcome of a crash at this exact point; the
	// system's job is to make this visible and recoverable, not to pretend
	// it can safely auto-decide a rollback with no evidence of the
	// candidate's health.
	badID := dockerImageID(t, fixtureBadImage)
	if got := dockerRunningImageID(t); got != badID {
		t.Fatalf("expected the container left on the unverified candidate image %s, got %s", badID, got)
	}

	manager := restartManager(t, dataDir, composeDir, fixtureRecoveredImage)
	if manager.Status().State != StateFailed || !strings.Contains(manager.Status().Message, "unterbrochen") {
		t.Fatalf("expected an interrupted-update diagnostic after restart, got %#v", manager.Status())
	}

	// The appliance must still be recoverable even from this worse
	// mid-flight state: a retried update backs up whatever is currently
	// running and swaps to a known-good image successfully.
	recoveredID := dockerImageID(t, fixtureRecoveredImage)
	if _, err := manager.StartUpdate("fixture"); err != nil {
		t.Fatal(err)
	}
	waitForIdleWithin(t, manager, 90*time.Second)
	if manager.Status().State != StateIdle {
		t.Fatalf("expected the retried update to succeed, got %#v", manager.Status())
	}
	if got := dockerRunningImageID(t); got != recoveredID {
		t.Fatalf("expected the container on the recovered image %s after retry, got %s", recoveredID, got)
	}
}
