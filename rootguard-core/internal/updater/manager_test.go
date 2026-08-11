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
