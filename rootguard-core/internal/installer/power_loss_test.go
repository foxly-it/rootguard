package installer

// Proves Manager survives a real process kill (simulating power loss or an
// OOM kill) between the discrete steps of a guided deployment, not just a
// torn file write - writeCompose/persistLocked already use atomic
// temp+rename, which this test does not need to re-prove in isolation.
// Since Go has no per-goroutine kill, the only way to genuinely simulate a
// killed process is to run the operation in a real child OS process and
// SIGKILL it - the same "TestHelperProcess" technique Go's own os/exec
// tests use, and the same one internal/updater's
// power_loss_integration_test.go uses for the update path.
//
// Deliberately Docker-free (Run/Bootstrap are mocked, matching the existing
// TestDeploymentPersistsCompletedState pattern) rather than a
// //go:build integration test against real containers: deploy() hardcodes
// production container/network names ("rootguard-unbound", "rootguard-dns"),
// so a real run would collide with an actual RootGuard deployment on any
// host that has one - unlike updater.Manager, there is no
// Options-configurable project name to isolate it with here, and adding one
// would be a materially bigger change than this test warrants. The
// process-persistence layer under test here - status.json/compose.yaml
// surviving a real kill, and NewManager detecting the interruption - does
// not depend on the containers being real.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestDeployHelperProcess is never run directly by the normal test suite (it
// self-skips unless ROOTGUARD_HELPER_PROCESS is set) - it is the child body
// launched by killDeployHelperAtCheckpoint below.
func TestDeployHelperProcess(t *testing.T) {
	if os.Getenv("ROOTGUARD_HELPER_PROCESS") != "1" {
		t.Skip("only runs as a subprocess helper for the power-loss test")
	}
	checkpoints := os.Getenv("ROOTGUARD_HELPER_CHECKPOINTS")
	touch := func(name string) {
		_ = os.WriteFile(filepath.Join(checkpoints, name), nil, 0600)
	}
	manager := NewManager(mockedDeployOptions(os.Getenv("ROOTGUARD_HELPER_DATA_DIR"), touch, true))
	if _, err := manager.Start(context.Background(), Config{DNSBindAddress: "192.168.1.2", DNSPort: 53}); err != nil {
		t.Fatal(err)
	}
	select {} // block until the parent SIGKILLs this process at the desired checkpoint
}

// mockedDeployOptions mirrors TestDeploymentPersistsCompletedState's mock
// Run (any command succeeds, "inspect" reports healthy) and additionally
// signals checkpoints right after the real "compose ... pull" and
// "compose ... up" steps persist as done, plus right when Bootstrap is
// invoked - the three points a real power loss most plausibly lands between.
// pauseAtCheckpoints must be true only for the killable helper process:
// everything here is otherwise mocked and effectively instant, so without a
// deliberate pause the whole deploy would finish before the parent's poll
// loop ever observes a checkpoint file, let alone kills the process while it
// is still genuinely "in progress" at that step. The restarted/retried
// Manager a test builds afterward must run at full (mocked) speed instead,
// or the artificial pauses would make the retry itself look stuck.
func mockedDeployOptions(dataDir string, touch func(name string), pauseAtCheckpoints bool) Options {
	return Options{
		DataDir: dataDir, CoreContainer: "rootguard-core",
		UnboundImage: "unbound:test", AdGuardImage: "adguard:test", DNSNetworkCIDR: "172.29.53.0/24",
		Run: func(_ context.Context, arguments ...string) ([]byte, error) {
			command := strings.Join(arguments, " ")
			switch {
			case arguments[0] == "inspect":
				return []byte("healthy\n"), nil
			case strings.Contains(command, "compose") && strings.HasSuffix(command, " pull"):
				touch("pull-done")
				if pauseAtCheckpoints {
					time.Sleep(5 * time.Second)
				}
			case strings.Contains(command, "compose") && strings.HasSuffix(command, " up -d"):
				touch("start-done")
				if pauseAtCheckpoints {
					time.Sleep(5 * time.Second)
				}
			}
			return []byte("ok"), nil
		},
		Bootstrap: func(context.Context, string) error {
			touch("bootstrap-called")
			return nil
		},
	}
}

// killDeployHelperAtCheckpoint launches the helper process, waits for it to
// signal the named checkpoint, then SIGKILLs it - simulating the host losing
// power at exactly that point in the deployment. It always waits for the
// process to actually exit before returning.
func killDeployHelperAtCheckpoint(t *testing.T, dataDir, checkpoint string) {
	t.Helper()
	checkpoints := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestDeployHelperProcess$", "-test.v")
	cmd.Env = append(os.Environ(),
		"ROOTGUARD_HELPER_PROCESS=1",
		"ROOTGUARD_HELPER_DATA_DIR="+dataDir,
		"ROOTGUARD_HELPER_CHECKPOINTS="+checkpoints,
	)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start helper process: %v", err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _, _ = cmd.Process.Wait() })

	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(checkpoints, checkpoint)); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("helper process never reached checkpoint %q", checkpoint)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatalf("kill helper process: %v", err)
	}
	_, _ = cmd.Process.Wait()
}

func restartInstallerManager(t *testing.T, dataDir string) *Manager {
	t.Helper()
	return NewManager(mockedDeployOptions(dataDir, func(string) {}, false))
}

func waitForInstalled(t *testing.T, manager *Manager) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if manager.Status().State == StateInstalled {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("deployment retry did not reach installed state, got %#v", manager.Status())
}

func TestDeployPowerLossDuringPullRecoversCleanly(t *testing.T) {
	dataDir := t.TempDir()
	killDeployHelperAtCheckpoint(t, dataDir, "pull-done")

	manager := restartInstallerManager(t, dataDir)
	status := manager.Status()
	if status.State != StateFailed || status.Diagnostic == nil || status.Diagnostic.Code != "deployment_interrupted" {
		t.Fatalf("expected an interrupted-deployment diagnostic after restart, got %#v", status)
	}
	// The compose file written just before the kill must still be intact -
	// proving writeCompose's atomic temp+rename held up under an actual
	// process kill, not just reasoning about the code.
	data, err := os.ReadFile(filepath.Join(dataDir, "compose.yaml"))
	if err != nil || len(data) == 0 {
		t.Fatalf("compose.yaml missing or empty after an interrupted deploy: %v", err)
	}

	// The appliance must not be stuck: a retried deployment now succeeds.
	if _, err := manager.Start(context.Background(), Config{DNSBindAddress: "192.168.1.2", DNSPort: 53}); err != nil {
		t.Fatal(err)
	}
	waitForInstalled(t, manager)
}

func TestDeployPowerLossAfterStartBeforeBootstrapRecoversCleanly(t *testing.T) {
	dataDir := t.TempDir()
	killDeployHelperAtCheckpoint(t, dataDir, "start-done")

	manager := restartInstallerManager(t, dataDir)
	status := manager.Status()
	if status.State != StateFailed || status.Diagnostic == nil || status.Diagnostic.Code != "deployment_interrupted" {
		t.Fatalf("expected an interrupted-deployment diagnostic after restart, got %#v", status)
	}

	// The appliance must still be recoverable even from this later
	// mid-flight state: a retried deployment redrives every step, including
	// the ones the killed attempt never reached, and completes successfully.
	if _, err := manager.Start(context.Background(), Config{DNSBindAddress: "192.168.1.2", DNSPort: 53}); err != nil {
		t.Fatal(err)
	}
	waitForInstalled(t, manager)
}
