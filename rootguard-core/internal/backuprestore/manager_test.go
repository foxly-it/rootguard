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

// TestFailedRestoreNormalizesOwnershipAfterRollingBackLocalData is the
// regression test for a round-3 correctness-review finding:
// copyDirectory (used to both back up and restore the local Unbound/
// AdGuard directories around a restore attempt) always writes 0600/0700
// regardless of the source's own mode, so files restored by the rollback
// path land root-owned (whatever UID this process runs as) at 0600 -
// unreadable by the unbound user (uid 100) Unbound's own container runs
// as. The success path doesn't have this problem only because it happens
// to call normalizeUnboundOwnership right after its own restoreData
// succeeds (chowning these same files to 100:101, after which 0600 is
// perfectly readable) - rolling back never got the same treatment.
// Forces restoreData to fail on the container-path "cp" step (after the
// local-directory replace already ran, but before restoreData reaches its
// own normalizeUnboundOwnership call) so the rollback path is what has to
// pick up the ownership fix.
func TestFailedRestoreNormalizesOwnershipAfterRollingBackLocalData(t *testing.T) {
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
	var commands []string
	var chownCallsAfterCpFailure int
	cpFailed := false
	docker := func(_ context.Context, arguments ...string) ([]byte, error) {
		mu.Lock()
		defer mu.Unlock()
		commands = append(commands, strings.Join(arguments, " "))
		if len(arguments) >= 2 && (arguments[0] == "container" || arguments[0] == "volume" || arguments[0] == "network") && arguments[1] == "inspect" {
			return []byte("not found"), errors.New("not found")
		}
		if arguments[0] == "cp" {
			cpFailed = true
			return nil, errors.New("simulated docker cp failure")
		}
		if arguments[0] == "run" && strings.Contains(strings.Join(arguments, " "), "100:101") && cpFailed {
			chownCallsAfterCpFailure++
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
		AttestationVerifier: func(context.Context, string, string) error { return nil },
	})
	target := t.TempDir()
	if err := os.WriteFile(filepath.Join(target, "old"), []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	manager := New(Options{DataDir: t.TempDir(), UnboundDir: target, AdGuardDir: t.TempDir(), AdGuardAuthDir: t.TempDir(), Installer: installerManager, Run: docker})
	_, err := manager.Restore(context.Background(), RestoreRequest{Passphrase: testPassphrase, Archive: bytes.NewReader(encrypted.Bytes())})
	if err == nil || !strings.Contains(err.Error(), "simulated docker cp failure") {
		t.Fatalf("expected the injected cp failure to surface, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(target, "old")); statErr != nil {
		t.Fatalf("expected the local directory to be rolled back to its pre-restore content: %v", statErr)
	}

	mu.Lock()
	defer mu.Unlock()
	if chownCallsAfterCpFailure == 0 {
		t.Fatalf("expected ownership to be normalized again after rolling back local data, got no post-failure chown call among:\n%s", strings.Join(commands, "\n"))
	}
}
