//go:build integration

// Exercises the real pre-update snapshot and rollback path - Manager.backup,
// Manager.rollback, and the SHA-256 checksum verification guarding it -
// against a real Docker container, a real docker-compose image swap, and a
// real docker cp, not a mocked CommandRunner. The only stub is skipping the
// "pull" step, since the fixture image is built locally and never pushed to
// a registry - the same accommodation rootguard-updater's own end-to-end
// scenario makes for its local fixture images (ROOTGUARD_UPDATER_SKIP_PULL).
// Requires Docker. Run with:
// go test -tags integration ./internal/updater/... -run TestRollback -v
package updater

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	fixtureContainer = "rootguard-updater-fixture-it"
	// A dedicated Compose project, deliberately distinct from
	// Manager.DefaultComposeProject ("rootguard-dns") - a real RootGuard
	// deployment can be running under that exact project name on any host
	// this test runs against, and reusing it here would let this test's
	// compose lifecycle interact with that unrelated live stack.
	fixtureComposeProject = "rootguard-updater-fixture-it"
	fixtureOldImage       = "rootguard-updater-fixture:old"
	fixtureBadImage       = "rootguard-updater-fixture:bad"
	// A second healthy tag, distinct from fixtureOldImage, so a recovery
	// retry after a simulated crash (power_loss_integration_test.go) proves
	// a real image swap and health check, not the same-ID "no_change"
	// fast path Manager.update takes when the target already matches.
	fixtureRecoveredImage = "rootguard-updater-fixture:recovered"
)

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

// waitForIdleWithin mirrors waitForIdle (manager_test.go) but with a timeout
// long enough for a real docker compose up, a real image build/swap, and
// real Docker HEALTHCHECK polling - the mocked unit tests' 2-second budget
// only ever has to wait out fake, in-process command handlers.
func waitForIdleWithin(t *testing.T, manager *Manager, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		state := manager.Status().State
		if state == StateIdle || state == StateFailed {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("operation did not finish")
}

func buildFixtureImages(t *testing.T) {
	t.Helper()
	buildFixture(t, fixtureOldImage, "true", "old")
	buildFixture(t, fixtureBadImage, "false", "bad")
	buildFixture(t, fixtureRecoveredImage, "true", "recovered")
}

func buildFixture(t *testing.T, tag, healthy, variant string) {
	t.Helper()
	output, err := exec.Command("docker", "build",
		"--build-arg", "HEALTHY="+healthy,
		"--build-arg", "VARIANT="+variant,
		"--tag", tag, "testdata/fixture").CombinedOutput()
	if err != nil {
		t.Fatalf("docker build %s: %v: %s", tag, err, output)
	}
}

// startFixtureContainer starts the fixture through docker compose, under the
// isolated fixtureComposeProject, using the same base compose.yaml Manager
// itself will later reuse for its own composeUp calls during update/rollback -
// production containers are always compose-managed, and recreating one that
// was instead started with a bare "docker run" fails with a name conflict
// (compose refuses to adopt a container it doesn't already track).
func startFixtureContainer(t *testing.T, composeDir, image string) {
	t.Helper()
	base := filepath.Join(composeDir, "compose.yaml")
	initialOverride := filepath.Join(t.TempDir(), "initial.yaml")
	content := "services:\n  fixture:\n    image: \"" + image + "\"\n"
	if err := os.WriteFile(initialOverride, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	compose := func(args ...string) ([]byte, error) {
		full := append([]string{"compose", "--project-name", fixtureComposeProject, "-f", base, "-f", initialOverride}, args...)
		return exec.Command("docker", full...).CombinedOutput()
	}
	if output, err := compose("up", "-d", "fixture"); err != nil {
		t.Fatalf("start fixture container: %v: %s", err, output)
	}
	t.Cleanup(func() {
		_, _ = compose("down", "--volumes")
	})
}

func writeFixtureCompose(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	compose := "services:\n  fixture:\n    container_name: " + fixtureContainer + "\n"
	if err := os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte(compose), 0600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func fixtureHealthVerify(ctx context.Context, _ string) error {
	output, err := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{.State.Health.Status}}", fixtureContainer).CombinedOutput()
	if err != nil {
		return err
	}
	if status := strings.TrimSpace(string(output)); status != "healthy" {
		return errors.New("fixture container not healthy: " + status)
	}
	return nil
}

func dockerImageID(t *testing.T, image string) string {
	t.Helper()
	output, err := exec.Command("docker", "image", "inspect", "--format", "{{.Id}}", image).CombinedOutput()
	if err != nil {
		t.Fatalf("inspect image %s: %v: %s", image, err, output)
	}
	return strings.TrimSpace(string(output))
}

func dockerRunningImageID(t *testing.T) string {
	t.Helper()
	output, err := exec.Command("docker", "inspect", "--format", "{{.Image}}", fixtureContainer).CombinedOutput()
	if err != nil {
		t.Fatalf("inspect running container: %v: %s", err, output)
	}
	return strings.TrimSpace(string(output))
}

func dockerReadFile(t *testing.T, path string) string {
	t.Helper()
	output, err := exec.Command("docker", "exec", fixtureContainer, "cat", path).CombinedOutput()
	if err != nil {
		t.Fatalf("read %s from fixture container: %v: %s", path, err, output)
	}
	return string(output)
}

// realDockerSkippingPull runs every command through the real docker CLI
// except "pull" (stubbed - the fixture image only exists locally) and, if
// afterBackup is set, invokes it with the real host-side backup directory
// right after the real backup docker-cp completes, so a test can tamper
// with an already-manifested snapshot exactly like real bit-rot or a
// partial copy would - the manifest reflects the pre-tamper content since
// it is written by Manager.backup before this hook ever runs.
func realDockerSkippingPull(afterBackup func(destination string)) CommandRunner {
	var backupDir string
	return func(ctx context.Context, arguments ...string) ([]byte, error) {
		if len(arguments) >= 3 && arguments[0] == "cp" && strings.HasPrefix(arguments[1], fixtureContainer+":") {
			output, err := runDocker(ctx, arguments...)
			if err == nil {
				backupDir = arguments[2]
			}
			return output, err
		}
		if len(arguments) > 0 && arguments[0] == "pull" {
			if afterBackup != nil && backupDir != "" {
				afterBackup(backupDir)
			}
			return []byte("skipped: local fixture image"), nil
		}
		return runDocker(ctx, arguments...)
	}
}

func TestRollbackRestoresCleanBackupAfterFailedHealthCheck(t *testing.T) {
	buildFixtureImages(t)
	composeDir := writeFixtureCompose(t)
	startFixtureContainer(t, composeDir, fixtureOldImage)
	oldID := dockerImageID(t, fixtureOldImage)

	manager := NewManager(Options{
		DataDir: t.TempDir(), ComposeDir: composeDir, ComposeProject: fixtureComposeProject,
		VerifyAttempts: 15, RetryDelay: time.Second,
		Run: realDockerSkippingPull(nil), Verify: fixtureHealthVerify,
		Services: []ServiceSpec{{
			Name: "fixture", Container: fixtureContainer,
			TargetImage: fixtureBadImage, BackupPaths: []string{"/data"},
		}},
	})

	if _, err := manager.StartUpdate("fixture"); err != nil {
		t.Fatal(err)
	}
	waitForIdleWithin(t, manager, 90*time.Second)
	result := manager.Status()
	if result.State != StateFailed || !strings.Contains(result.Message, "rolled back safely") {
		t.Fatalf("expected a safe rollback, got %#v", result)
	}
	if got := dockerRunningImageID(t); got != oldID {
		t.Fatalf("expected the container back on the previous image %s, got %s", oldID, got)
	}
	if content := dockerReadFile(t, "/data/marker"); content != "authentic data" {
		t.Fatalf("real backup content did not survive a real rollback: %q", content)
	}
}

func TestRollbackRefusesTamperedBackupAgainstRealContainer(t *testing.T) {
	buildFixtureImages(t)
	composeDir := writeFixtureCompose(t)
	startFixtureContainer(t, composeDir, fixtureOldImage)
	oldID := dockerImageID(t, fixtureOldImage)

	tamper := func(destination string) {
		marker := filepath.Join(destination, "data", "marker")
		if err := os.WriteFile(marker, []byte("tampered"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	manager := NewManager(Options{
		DataDir: t.TempDir(), ComposeDir: composeDir, ComposeProject: fixtureComposeProject,
		VerifyAttempts: 15, RetryDelay: time.Second,
		Run: realDockerSkippingPull(tamper), Verify: fixtureHealthVerify,
		Services: []ServiceSpec{{
			Name: "fixture", Container: fixtureContainer,
			TargetImage: fixtureBadImage, BackupPaths: []string{"/data"},
		}},
	})

	if _, err := manager.StartUpdate("fixture"); err != nil {
		t.Fatal(err)
	}
	waitForIdleWithin(t, manager, 90*time.Second)
	result := manager.Status()
	if result.State != StateFailed || !strings.Contains(result.Message, "verify backup integrity") {
		t.Fatalf("expected a refused rollback due to backup integrity, got %#v", result)
	}
	// The container must still land back on the known-good previous image
	// even though the tampered data restore was refused - see the ordering
	// comment in Manager.rollback.
	if got := dockerRunningImageID(t); got != oldID {
		t.Fatalf("expected the container back on the previous image %s despite the refused restore, got %s", oldID, got)
	}
	if content := dockerReadFile(t, "/data/marker"); content != "authentic data" {
		t.Fatalf("fixture container content changed despite the refused restore: %q", content)
	}
}
