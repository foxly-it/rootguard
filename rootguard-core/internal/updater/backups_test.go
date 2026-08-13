package updater

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUpdateEnforcesBackupRetentionAfterLifecycle(t *testing.T) {
	dataDir := t.TempDir()
	composeDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(composeDir, "compose.yaml"), []byte("services: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	oldest := writeManagedBackup(t, dataDir, "20260801T010000.000000000Z", "unbound", "rootguard-unbound", "oldest")
	for day := 2; day <= DefaultBackupRetention; day++ {
		writeManagedBackup(t, dataDir, "2026080"+string(rune('0'+day))+"T010000.000000000Z", "unbound", "rootguard-unbound", "existing")
	}
	manager := NewManager(Options{
		DataDir: dataDir, ComposeDir: composeDir,
		Services: []ServiceSpec{{
			Name: "unbound", DisplayName: "Unbound", Container: "rootguard-unbound",
			TargetImage: "rootguard-unbound:latest", BackupPaths: []string{"/etc/unbound/unbound.d"},
		}},
		Run: func(_ context.Context, arguments ...string) ([]byte, error) {
			switch arguments[0] {
			case "inspect":
				return []byte("rootguard-unbound:v1|sha256:old"), nil
			case "image":
				return []byte("sha256:old"), nil
			default:
				return []byte("ok"), nil
			}
		},
	})

	if _, err := manager.StartUpdate("unbound"); err != nil {
		t.Fatal(err)
	}
	waitForIdle(t, manager)
	status, err := manager.BackupStatus()
	if err != nil {
		t.Fatal(err)
	}
	if status.Count != DefaultBackupRetention {
		t.Fatalf("expected %d retained backups, got %+v", DefaultBackupRetention, status)
	}
	if _, err := os.Stat(oldest); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("oldest backup was not pruned after update lifecycle: %v", err)
	}
}

func TestBackupStatusAndRetention(t *testing.T) {
	dataDir := t.TempDir()
	manager := NewManager(Options{
		DataDir: dataDir,
		Services: []ServiceSpec{
			{Name: "adguard", Container: "rootguard-adguard"},
			{Name: "unbound", Container: "rootguard-unbound"},
		},
	})
	oldest := writeManagedBackup(t, dataDir, "20260801T010000.000000000Z", "unbound", "rootguard-unbound", "oldest")
	second := writeManagedBackup(t, dataDir, "20260802T010000.000000000Z", "unbound", "rootguard-unbound", "second")
	writeManagedBackup(t, dataDir, "20260803T010000.000000000Z", "unbound", "rootguard-unbound", "third")
	writeManagedBackup(t, dataDir, "20260804T010000.000000000Z", "unbound", "rootguard-unbound", "newest")
	writeManagedBackup(t, dataDir, "20260804T020000.000000000Z", "adguard", "rootguard-adguard", "adguard")
	foreign := filepath.Join(dataDir, "backups", "foreign")
	if err := os.MkdirAll(foreign, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(foreign, "keep"), []byte("foreign"), 0600); err != nil {
		t.Fatal(err)
	}

	status, err := manager.BackupStatus()
	if err != nil {
		t.Fatal(err)
	}
	if status.Settings.RetentionPerService != DefaultBackupRetention || status.Count != 5 || status.ManagedBytes == 0 || status.UnmanagedBytes != 7 {
		t.Fatalf("unexpected initial status: %+v", status)
	}
	status, err = manager.SetBackupRetention(2)
	if err != nil {
		t.Fatal(err)
	}
	if status.Count != 3 || status.Settings.RetentionPerService != 2 {
		t.Fatalf("unexpected pruned status: %+v", status)
	}
	for _, removed := range []string{oldest, second} {
		if _, err := os.Stat(removed); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expected %q to be removed, got %v", removed, err)
		}
	}
	if _, err := os.Stat(foreign); err != nil {
		t.Fatalf("foreign data was removed: %v", err)
	}

	reloaded := NewManager(Options{DataDir: dataDir, Services: []ServiceSpec{{Name: "unbound", Container: "rootguard-unbound"}}})
	reloadedStatus, err := reloaded.BackupStatus()
	if err != nil || reloadedStatus.Settings.RetentionPerService != 2 {
		t.Fatalf("retention was not persisted: status=%+v err=%v", reloadedStatus, err)
	}
}

func TestBackupRetentionRejectsUnsafeValuesAndBusyManager(t *testing.T) {
	manager := NewManager(Options{DataDir: t.TempDir()})
	for _, retention := range []int{MinBackupRetention - 1, MaxBackupRetention + 1} {
		if _, err := manager.SetBackupRetention(retention); !errors.Is(err, ErrInvalidBackupRetention) {
			t.Fatalf("expected invalid retention for %d, got %v", retention, err)
		}
	}
	manager.mu.Lock()
	manager.status.State = StateUpdating
	manager.mu.Unlock()
	if _, err := manager.SetBackupRetention(MinBackupRetention); !errors.Is(err, ErrBusy) {
		t.Fatalf("expected busy error, got %v", err)
	}
}

func TestBackupScannerDoesNotFollowSymlinksOrTrustMismatchedManifests(t *testing.T) {
	dataDir := t.TempDir()
	manager := NewManager(Options{DataDir: dataDir, Services: []ServiceSpec{{Name: "unbound", Container: "rootguard-unbound"}}})
	timestamp := "20260804T010000.000000000Z"
	writeManagedBackup(t, dataDir, timestamp, "unbound", "different-container", "untrusted")
	external := t.TempDir()
	if err := os.WriteFile(filepath.Join(external, "secret"), []byte("do-not-follow"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dataDir, "backups", "20260805T010000.000000000Z")
	if err := os.Symlink(external, link); err != nil {
		t.Fatal(err)
	}

	status, err := manager.SetBackupRetention(MinBackupRetention)
	if err != nil {
		t.Fatal(err)
	}
	if status.Count != 0 || status.UnmanagedBytes == 0 {
		t.Fatalf("unsafe entries were not isolated: %+v", status)
	}
	if _, err := os.Stat(filepath.Join(external, "secret")); err != nil {
		t.Fatalf("symlink target was modified: %v", err)
	}
}

func TestWriteAndVerifyBackupManifestRoundTrips(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "AdGuardHome.yaml"), []byte("original configuration"), 0600); err != nil {
		t.Fatal(err)
	}
	spec := ServiceSpec{Name: "adguard", Container: "rootguard-adguard", TargetImage: "adguard:latest"}
	if err := writeBackupManifest(directory, spec); err != nil {
		t.Fatal(err)
	}
	if err := verifyBackupManifest(directory, spec); err != nil {
		t.Fatalf("expected an untouched backup to verify, got %v", err)
	}
}

func TestVerifyBackupManifestDetectsTamperedContent(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "AdGuardHome.yaml"), []byte("original configuration"), 0600); err != nil {
		t.Fatal(err)
	}
	spec := ServiceSpec{Name: "adguard", Container: "rootguard-adguard"}
	if err := writeBackupManifest(directory, spec); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "AdGuardHome.yaml"), []byte("tampered"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := verifyBackupManifest(directory, spec); err == nil {
		t.Fatal("expected tampered file content to fail verification")
	}
}

func TestVerifyBackupManifestDetectsMissingAndExtraFiles(t *testing.T) {
	spec := ServiceSpec{Name: "adguard", Container: "rootguard-adguard"}

	missing := t.TempDir()
	if err := os.WriteFile(filepath.Join(missing, "AdGuardHome.yaml"), []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := writeBackupManifest(missing, spec); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(missing, "AdGuardHome.yaml")); err != nil {
		t.Fatal(err)
	}
	if err := verifyBackupManifest(missing, spec); err == nil {
		t.Fatal("expected a missing backup file to fail verification")
	}

	extra := t.TempDir()
	if err := os.WriteFile(filepath.Join(extra, "AdGuardHome.yaml"), []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := writeBackupManifest(extra, spec); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(extra, "unexpected"), []byte("planted"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := verifyBackupManifest(extra, spec); err == nil {
		t.Fatal("expected an unexpected extra backup file to fail verification")
	}
}

func TestVerifyBackupManifestRefusesSymlinksAndMismatchedService(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "AdGuardHome.yaml"), []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	spec := ServiceSpec{Name: "adguard", Container: "rootguard-adguard"}
	if err := writeBackupManifest(directory, spec); err != nil {
		t.Fatal(err)
	}

	if err := verifyBackupManifest(directory, ServiceSpec{Name: "unbound", Container: "rootguard-unbound"}); err == nil {
		t.Fatal("expected a manifest for a different service to fail verification")
	}

	external := t.TempDir()
	if err := os.WriteFile(filepath.Join(external, "secret"), []byte("do-not-follow"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(directory, "linked")); err != nil {
		t.Fatal(err)
	}
	if err := verifyBackupManifest(directory, spec); err == nil {
		t.Fatal("expected a symlinked backup entry to fail verification")
	}
}

func TestVerifyBackupManifestRejectsUnsupportedSchemaVersion(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "AdGuardHome.yaml"), []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	spec := ServiceSpec{Name: "adguard", Container: "rootguard-adguard"}
	if err := writeBackupManifest(directory, spec); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(directory, manifestFileName))
	if err != nil {
		t.Fatal(err)
	}
	var manifest backupManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	manifest.SchemaVersion = backupManifestSchemaVersion + 1
	data, err = json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, manifestFileName), data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := verifyBackupManifest(directory, spec); err == nil {
		t.Fatal("expected an unsupported schema version to fail verification")
	}
}

func writeManagedBackup(t *testing.T, dataDir, timestamp, service, container, content string) string {
	t.Helper()
	directory := filepath.Join(dataDir, "backups", timestamp, service)
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	manifest, err := json.Marshal(map[string]any{"schema_version": backupManifestSchemaVersion, "service": service, "container": container, "image": "test"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "manifest.json"), manifest, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "payload"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := time.Parse(backupTimestampLayout, timestamp); err != nil {
		t.Fatal(err)
	}
	return directory
}
