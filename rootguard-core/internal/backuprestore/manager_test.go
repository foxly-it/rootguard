package backuprestore

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/foxly-it/rootguard-core/internal/backupexport"
	"github.com/foxly-it/rootguard-core/internal/installer"
)

func TestManagerRestoresVerifiedDataThroughCleanInstaller(t *testing.T) {
	installation := t.TempDir()
	status := `{"state":"installed","config":{"dns_bind_address":"192.0.2.10","dns_port":53,"adguard_channel":"stable","blockpage_enabled":false}}`
	if err := os.WriteFile(filepath.Join(installation, "status.json"), []byte(status), 0600); err != nil {
		t.Fatal(err)
	}
	unbound := t.TempDir()
	if err := os.WriteFile(filepath.Join(unbound, "settings.json"), []byte("restored"), 0600); err != nil {
		t.Fatal(err)
	}
	service := t.TempDir()
	if err := os.WriteFile(filepath.Join(service, "state"), []byte("service-state"), 0600); err != nil {
		t.Fatal(err)
	}
	credentials := t.TempDir()
	if err := os.WriteFile(filepath.Join(credentials, "credentials.json"), []byte(`{"username":"rootguard","password":"secret"}`), 0600); err != nil {
		t.Fatal(err)
	}
	adguard := t.TempDir()
	if err := os.WriteFile(filepath.Join(adguard, "AdGuardHome.yaml"), []byte("schema_version: 29"), 0600); err != nil {
		t.Fatal(err)
	}
	exporter := backupexport.New(backupexport.Options{DataDir: t.TempDir(), LocalSources: []backupexport.Source{
		{ArchivePath: "rootguard/installation", Path: installation},
		{ArchivePath: "rootguard/unbound", Path: unbound},
		{ArchivePath: "rootguard/adguard", Path: credentials},
		{ArchivePath: "services/adguard/config", Path: adguard},
		{ArchivePath: "services/unbound/state", Path: service},
	}})
	var encrypted bytes.Buffer
	if err := exporter.Export(context.Background(), testPassphrase, &encrypted); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	commands := []string{}
	docker := func(_ context.Context, arguments ...string) ([]byte, error) {
		mu.Lock()
		commands = append(commands, strings.Join(arguments, " "))
		mu.Unlock()
		if len(arguments) >= 2 && (arguments[0] == "container" || arguments[0] == "volume" || arguments[0] == "network") && arguments[1] == "inspect" {
			return []byte("not found"), errors.New("not found")
		}
		if len(arguments) > 2 && arguments[0] == "inspect" && arguments[1] == "--format" {
			if arguments[len(arguments)-1] == "rootguard-unbound" && arguments[2] == "{{.Config.Image}}" {
				return []byte("unbound:test"), nil
			}
			return []byte("healthy"), nil
		}
		return nil, nil
	}
	installerManager := installer.NewManager(installer.Options{
		DataDir: t.TempDir(), CoreContainer: "rootguard-core", DNSNetworkCIDR: "172.29.53.0/24", Run: docker,
		// Not about attestation itself - stack.RequireAttestation (the
		// real default) fails closed on a non-matching image for any
		// service with a real signing policy, "unbound" included, which
		// this test's fixture image is.
		AttestationVerifier: func(context.Context, string, string) error { return nil },
	})
	target := t.TempDir()
	if err := os.WriteFile(filepath.Join(target, "old"), []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	manager := New(Options{DataDir: t.TempDir(), UnboundDir: target, AdGuardDir: t.TempDir(), AdGuardAuthDir: t.TempDir(), Installer: installerManager, Run: docker})
	result, err := manager.Restore(context.Background(), RestoreRequest{Passphrase: testPassphrase, Archive: bytes.NewReader(encrypted.Bytes())})
	if err != nil {
		t.Fatal(err)
	}
	if result.State != installer.StateInstalled {
		t.Fatalf("unexpected status: %+v", result)
	}
	data, err := os.ReadFile(filepath.Join(target, "settings.json"))
	if err != nil || string(data) != "restored" {
		t.Fatalf("local data not restored: %q %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(target, "old")); !os.IsNotExist(err) {
		t.Fatal("old clean-target data was retained")
	}
	joined := strings.Join(commands, "\n")
	for _, expected := range []string{"compose --project-name rootguard-dns", " create", "cp ", "rootguard-unbound:/var/lib/unbound", "rootguard-unbound-config:/etc/unbound/unbound.d", "100:101", " up -d", "network connect --ip 172.29.53.5"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("missing command %q in:\n%s", expected, joined)
		}
	}
}

// TestFailedPreflightLeavesLocalDirectoriesUntouched is the regression test
// for a v1.0.0 correctness review finding: ErrNotClean means
// m.installer.Restore refused before ever invoking the restoreData
// callback (RestorePreflight failed), so none of the local directories
// were ever actually replaced. The rollback step used to run
// unconditionally on any restoreErr anyway, "restoring" each target from
// the backup this function had just staged of that same, never-touched
// directory - copyDirectory always writes 0600/0700 regardless of the
// original mode, so this pointless rollback silently stripped the
// target's real permissions for no reason.
func TestFailedPreflightLeavesLocalDirectoriesUntouched(t *testing.T) {
	installation := t.TempDir()
	status := `{"state":"installed","config":{"dns_bind_address":"192.0.2.10","dns_port":53,"adguard_channel":"stable","blockpage_enabled":false}}`
	if err := os.WriteFile(filepath.Join(installation, "status.json"), []byte(status), 0600); err != nil {
		t.Fatal(err)
	}
	unbound := t.TempDir()
	if err := os.WriteFile(filepath.Join(unbound, "settings.json"), []byte("archived"), 0600); err != nil {
		t.Fatal(err)
	}
	service := t.TempDir()
	if err := os.WriteFile(filepath.Join(service, "state"), []byte("service-state"), 0600); err != nil {
		t.Fatal(err)
	}
	credentials := t.TempDir()
	if err := os.WriteFile(filepath.Join(credentials, "credentials.json"), []byte(`{"username":"rootguard","password":"secret"}`), 0600); err != nil {
		t.Fatal(err)
	}
	adguard := t.TempDir()
	if err := os.WriteFile(filepath.Join(adguard, "AdGuardHome.yaml"), []byte("schema_version: 29"), 0600); err != nil {
		t.Fatal(err)
	}
	exporter := backupexport.New(backupexport.Options{DataDir: t.TempDir(), LocalSources: []backupexport.Source{
		{ArchivePath: "rootguard/installation", Path: installation},
		{ArchivePath: "rootguard/unbound", Path: unbound},
		{ArchivePath: "rootguard/adguard", Path: credentials},
		{ArchivePath: "services/adguard/config", Path: adguard},
		{ArchivePath: "services/unbound/state", Path: service},
	}})
	var encrypted bytes.Buffer
	if err := exporter.Export(context.Background(), testPassphrase, &encrypted); err != nil {
		t.Fatal(err)
	}

	// Every managed resource reports as present (inspect succeeds, no
	// "not found"/"no such" in its output) - RestorePreflight's own
	// resource-absence checks all fail, so Ready is false and Restore
	// refuses with ErrNotClean before ever calling restoreDeploy.
	docker := func(_ context.Context, arguments ...string) ([]byte, error) {
		if len(arguments) >= 2 && (arguments[0] == "container" || arguments[0] == "volume" || arguments[0] == "network") && arguments[1] == "inspect" {
			return []byte("already exists"), nil
		}
		return nil, nil
	}
	installerManager := installer.NewManager(installer.Options{
		DataDir: t.TempDir(), CoreContainer: "rootguard-core", DNSNetworkCIDR: "172.29.53.0/24", Run: docker,
		AttestationVerifier: func(context.Context, string, string) error { return nil },
	})
	target := t.TempDir()
	targetFile := filepath.Join(target, "settings.json")
	if err := os.WriteFile(targetFile, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	manager := New(Options{DataDir: t.TempDir(), UnboundDir: target, AdGuardDir: t.TempDir(), AdGuardAuthDir: t.TempDir(), Installer: installerManager, Run: docker})
	_, err := manager.Restore(context.Background(), RestoreRequest{Passphrase: testPassphrase, Archive: bytes.NewReader(encrypted.Bytes())})
	if !errors.Is(err, installer.ErrNotClean) {
		t.Fatalf("expected ErrNotClean, got %v", err)
	}

	data, err := os.ReadFile(targetFile)
	if err != nil || string(data) != "original" {
		t.Fatalf("expected untouched local data to survive a refused restore, got %q %v", data, err)
	}
	info, err := os.Stat(targetFile)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0644 {
		t.Fatalf("expected the untouched file's original mode 0644 to survive, got %o - a pointless rollback overwrote it", info.Mode().Perm())
	}
}
