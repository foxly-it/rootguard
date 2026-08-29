package installer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestPreflightRejectsInvalidNetworkValues(t *testing.T) {
	manager := NewManager(Options{
		DataDir: t.TempDir(),
		Run: func(_ context.Context, _ ...string) ([]byte, error) {
			return []byte("ok"), nil
		},
	})

	report := manager.Preflight(context.Background(), Config{
		DNSBindAddress: "not-an-ip",
		DNSPort:        70000,
	})

	if report.Ready {
		t.Fatal("expected preflight to reject invalid settings")
	}
	if len(report.Checks) != 6 {
		t.Fatalf("expected six checks, got %d", len(report.Checks))
	}
}

func TestPreflightDefaultsAdGuardChannelToStable(t *testing.T) {
	manager := NewManager(Options{DataDir: t.TempDir(), Run: successfulDockerRun})
	report := manager.Preflight(context.Background(), Config{DNSBindAddress: "192.168.1.2", DNSPort: 53})
	if !report.Ready || report.Config.AdGuardChannel != "stable" {
		t.Fatalf("expected stable default, got %#v", report)
	}
}

func TestPreflightRejectsUnknownAdGuardChannel(t *testing.T) {
	manager := NewManager(Options{DataDir: t.TempDir(), Run: successfulDockerRun})
	report := manager.Preflight(context.Background(), Config{DNSBindAddress: "192.168.1.2", DNSPort: 53, AdGuardChannel: "edge"})
	if report.Ready || report.Checks[0].Code != "invalid_adguard_channel" {
		t.Fatalf("expected rejected channel, got %#v", report)
	}
}

func TestWriteComposeSelectsBetaImage(t *testing.T) {
	manager := NewManager(Options{
		DataDir: t.TempDir(), UnboundImage: "unbound:test", AdGuardImage: "adguard:stable",
		AdGuardBetaImage: "adguard:beta", DNSNetworkCIDR: "172.29.53.0/24",
	})
	path, err := manager.writeCompose(Config{DNSBindAddress: "0.0.0.0", DNSPort: 53, AdGuardChannel: "beta"})
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "image: adguard:beta") || strings.Contains(string(content), "image: adguard:stable") {
		t.Fatalf("expected beta image, got:\n%s", content)
	}
}

// noopAttestationVerifier lets tests that aren't specifically about the
// attestation gate itself exercise a real deploy()/restoreDeploy() flow
// without needing actual cosign infrastructure - stack.RequireAttestation
// (the real default) fails closed on a non-matching image for any
// service with a real signing policy, "unbound"/"blockpage" included,
// which every fixture image here is.
func noopAttestationVerifier(context.Context, string, string) error { return nil }

func successfulDockerRun(_ context.Context, arguments ...string) ([]byte, error) {
	if arguments[0] == "ps" {
		return []byte(""), nil
	}
	return []byte("ok"), nil
}

func TestInitialStatusUsesEmptyStepsArray(t *testing.T) {
	manager := NewManager(Options{DataDir: t.TempDir()})
	status := manager.Status()
	if status.Steps == nil {
		t.Fatal("expected an empty steps array, got nil")
	}
}

func TestPreflightRequiresDockerAndCompose(t *testing.T) {
	manager := NewManager(Options{
		DataDir: t.TempDir(),
		Run: func(_ context.Context, arguments ...string) ([]byte, error) {
			if arguments[0] == "compose" {
				return nil, errors.New("missing")
			}
			return []byte("ok"), nil
		},
	})

	report := manager.Preflight(context.Background(), Config{
		DNSBindAddress: "192.168.1.2",
		DNSPort:        53,
	})

	if report.Ready {
		t.Fatal("expected missing compose plugin to fail preflight")
	}
}

func TestPreflightReportsOccupiedDockerDNSPort(t *testing.T) {
	manager := NewManager(Options{
		DataDir: t.TempDir(),
		Run: func(_ context.Context, arguments ...string) ([]byte, error) {
			if arguments[0] == "ps" {
				return []byte("existing-dns|0.0.0.0:53->53/tcp, 0.0.0.0:53->53/udp\n"), nil
			}
			return []byte("ok"), nil
		},
	})

	report := manager.Preflight(context.Background(), Config{
		DNSBindAddress: "192.168.1.2",
		DNSPort:        53,
	})
	if report.Ready {
		t.Fatal("expected occupied DNS port to fail preflight")
	}
	check := report.Checks[len(report.Checks)-1]
	if check.Code != "dns_port_occupied" || check.Detail != "existing-dns" || check.Action == "" {
		t.Fatalf("unexpected occupied-port diagnostic: %#v", check)
	}
}

// TestPreflightProbesHostPortWhenDockerPsIsClean covers a `docker ps` blind
// spot: a non-Docker process (e.g. systemd-resolved's stub listener) can
// hold the requested port without ever showing up as a container. The
// real-publish probe must catch it even though the docker-ps-based check
// alone would have reported the port free.
//
// The fake runner deliberately returns a generic error ("exit status 1")
// with the actual diagnostic text only in the output bytes, mirroring
// os/exec's CombinedOutput() split - the production runDocker happens to
// fold output into err.Error() too, but the CommandRunner contract doesn't
// require that, so the classification must work from output alone.
func TestPreflightProbesHostPortWhenDockerPsIsClean(t *testing.T) {
	manager := NewManager(Options{
		DataDir:       t.TempDir(),
		CoreContainer: "rootguard-core",
		Run: func(_ context.Context, arguments ...string) ([]byte, error) {
			switch {
			case arguments[0] == "ps":
				return []byte(""), nil
			case arguments[0] == "run":
				return []byte("Bind for 0.0.0.0:53 failed: port is already allocated"), errors.New("exit status 1")
			}
			return []byte("ok"), nil
		},
	})

	report := manager.Preflight(context.Background(), Config{
		DNSBindAddress: "192.168.1.2",
		DNSPort:        53,
	})
	if report.Ready {
		t.Fatal("expected the host-port probe to fail preflight")
	}
	check := report.Checks[len(report.Checks)-1]
	if check.Code != "dns_port_occupied" || check.Detail == "" || check.Action == "" {
		t.Fatalf("unexpected probe diagnostic: %#v", check)
	}
}

// TestPreflightPassesWhenHostPortIsFree ensures the added probe doesn't
// introduce a false positive on the ordinary clean-host path.
func TestPreflightPassesWhenHostPortIsFree(t *testing.T) {
	manager := NewManager(Options{DataDir: t.TempDir(), CoreContainer: "rootguard-core", Run: successfulDockerRun})
	report := manager.Preflight(context.Background(), Config{DNSBindAddress: "192.168.1.2", DNSPort: 53})
	if !report.Ready {
		t.Fatalf("expected a free port to pass preflight, got %#v", report)
	}
}

// TestPreflightReportsOccupiedBlockpagePort is the regression test for a
// gap found in review: Preflight only ever checked the DNS port, so a host
// with something already bound to :80 (a common case - many hosts run a web
// server) was reported "ready", then failed only later, during deployment,
// when the blockpage container's own port publish collided with it. Mirrors
// TestPreflightReportsOccupiedDockerDNSPort, but for the blockpage's fixed
// port 80 with BlockpageEnabled set - the setting that actually gates
// whether that port gets published at all (see composeDNSFile).
func TestPreflightReportsOccupiedBlockpagePort(t *testing.T) {
	manager := NewManager(Options{
		DataDir: t.TempDir(),
		Run: func(_ context.Context, arguments ...string) ([]byte, error) {
			if arguments[0] == "ps" {
				return []byte("existing-web|0.0.0.0:80->80/tcp\n"), nil
			}
			return []byte("ok"), nil
		},
	})

	report := manager.Preflight(context.Background(), Config{
		DNSBindAddress:   "192.168.1.2",
		DNSPort:          53,
		BlockpageEnabled: true,
	})
	if report.Ready {
		t.Fatal("expected occupied blockpage port to fail preflight")
	}
	check := report.Checks[len(report.Checks)-1]
	if check.Code != "blockpage_port_occupied" || check.Detail != "existing-web" || check.Action == "" {
		t.Fatalf("unexpected occupied-blockpage-port diagnostic: %#v", check)
	}
}

// TestPreflightSkipsBlockpagePortCheckWhenDisabled ensures a host with
// something on port 80 doesn't fail preflight when the blockpage - the only
// thing that would ever publish that port - isn't even enabled.
func TestPreflightSkipsBlockpagePortCheckWhenDisabled(t *testing.T) {
	manager := NewManager(Options{
		DataDir: t.TempDir(),
		Run: func(_ context.Context, arguments ...string) ([]byte, error) {
			if arguments[0] == "ps" {
				return []byte("existing-web|0.0.0.0:80->80/tcp\n"), nil
			}
			return []byte("ok"), nil
		},
	})

	report := manager.Preflight(context.Background(), Config{
		DNSBindAddress:   "192.168.1.2",
		DNSPort:          53,
		BlockpageEnabled: false,
	})
	if !report.Ready {
		t.Fatalf("expected a disabled blockpage to skip the port-80 check, got %#v", report)
	}
}

// TestRunComposeUpRetriesTransientPortBindConflict covers the race a static
// `docker ps` preflight check cannot rule out: a container that just
// stopped can hold its published port for a moment after it's gone from
// `docker ps`, so the very next `up -d` on that port needs a short retry
// instead of failing the whole deployment outright.
//
// As in TestPreflightProbesHostPortWhenDockerPsIsClean, the diagnostic text
// lives only in the output bytes, not err.Error(), to prove the retry
// doesn't depend on the production runner's convention of folding output
// into the error text.
func TestRunComposeUpRetriesTransientPortBindConflict(t *testing.T) {
	var attempts int
	manager := NewManager(Options{
		DataDir:                t.TempDir(),
		ComposeUpRetryAttempts: 3,
		ComposeUpRetryDelay:    time.Millisecond,
		Run: func(_ context.Context, _ ...string) ([]byte, error) {
			attempts++
			if attempts < 3 {
				return []byte("Bind for 0.0.0.0:53 failed: port is already allocated"), errors.New("exit status 1")
			}
			return []byte("ok"), nil
		},
	})

	if _, err := manager.runComposeUp(context.Background(), "compose", "up", "-d"); err != nil {
		t.Fatalf("expected the third attempt to succeed, got: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected exactly 3 attempts, got %d", attempts)
	}
}

// TestRunComposeUpDoesNotRetryUnrelatedErrors ensures the retry is scoped
// to the specific transient port-bind race - any other failure (a bad
// image reference, a misconfigured mount, ...) must fail immediately
// rather than silently retrying and delaying the user's error by seconds.
func TestRunComposeUpDoesNotRetryUnrelatedErrors(t *testing.T) {
	var attempts int
	manager := NewManager(Options{
		DataDir:                t.TempDir(),
		ComposeUpRetryAttempts: 3,
		ComposeUpRetryDelay:    time.Millisecond,
		Run: func(_ context.Context, _ ...string) ([]byte, error) {
			attempts++
			return nil, errors.New("no such image: adguard:missing")
		},
	})

	if _, err := manager.runComposeUp(context.Background(), "compose", "up", "-d"); err == nil {
		t.Fatal("expected the unrelated error to be returned")
	}
	if attempts != 1 {
		t.Fatalf("expected exactly 1 attempt for a non-port-bind error, got %d", attempts)
	}
}

func TestDeploymentErrorsUseStableDiagnosticCodes(t *testing.T) {
	tests := []struct {
		phase string
		err   error
		code  string
	}{
		{"pull", errors.New("registry unavailable"), "image_pull_failed"},
		{"start", errors.New("Bind for 0.0.0.0:53 failed: port is already allocated"), "dns_port_occupied"},
		{"start", errors.New("listen tcp: cannot assign requested address"), "host_address_unavailable"},
		{"prepare", errors.New("read-only file system"), "state_write_failed"},
	}
	for _, test := range tests {
		diagnostic := classifyDeploymentError(test.phase, test.err)
		if diagnostic.Code != test.code || diagnostic.Phase != test.phase || diagnostic.Action == "" || !diagnostic.Retryable {
			t.Fatalf("unexpected diagnostic for %s: %#v", test.code, diagnostic)
		}
	}
}

func TestInterruptedDeploymentGetsRecoverableDiagnostic(t *testing.T) {
	dataDir := t.TempDir()
	data := `{"state":"deploying","steps":[{"id":"pull","status":"running","message":"pulling"}],"updated_at":"2026-07-28T00:00:00Z"}`
	if err := os.WriteFile(filepath.Join(dataDir, "status.json"), []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(Options{DataDir: dataDir})
	status := manager.Status()
	if status.State != StateFailed || status.Diagnostic == nil ||
		status.Diagnostic.Code != "deployment_interrupted" || !status.Diagnostic.Retryable {
		t.Fatalf("unexpected interrupted status: %#v", status)
	}
}

func TestRenderedComposeKeepsAdministrationPrivate(t *testing.T) {
	content, err := renderCompose(
		Config{DNSBindAddress: "192.168.1.2", DNSPort: 53},
		"ghcr.io/foxly-it/rootguard-unbound:latest",
		"adguard/adguardhome:v0.107.78",
		"ghcr.io/foxly-it/rootguard-blockpage:latest",
		"172.29.53.0/24",
	)
	if err != nil {
		t.Fatal(err)
	}

	for _, expected := range []string{
		`"192.168.1.2:53:53/tcp"`,
		`"192.168.1.2:53:53/udp"`,
		`io.rootguard.managed: "true"`,
		`external: true`,
		// AdGuard needs its own pinned address, not a dynamically assigned
		// one - unpinned, it can grab unbound's reserved 172.29.53.2 slot
		// whenever that happens to be free (e.g. while unbound is being
		// recreated during an update), permanently blocking unbound from
		// reclaiming its own statically configured address.
		"ipv4_address: 172.29.53.4",
	} {
		if !strings.Contains(content, expected) {
			t.Fatalf("rendered compose is missing %q", expected)
		}
	}
	if strings.Contains(content, "3000:3000") || strings.Contains(content, "80:80") {
		t.Fatal("AdGuard administration must not be published")
	}
}

func TestRenderedComposeBlockpageIsOptional(t *testing.T) {
	enabled, err := renderCompose(
		Config{DNSBindAddress: "192.168.1.2", DNSPort: 53, BlockpageEnabled: true},
		"rootguard-unbound:test",
		"adguard:test",
		"ghcr.io/foxly-it/rootguard-blockpage:latest",
		"172.29.53.0/24",
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"rootguard-blockpage",
		"ghcr.io/foxly-it/rootguard-blockpage:latest",
		`"192.168.1.2:80:8080/tcp"`,
		`io.rootguard.component: "blockpage"`,
		"cap_add: [CHOWN, SETUID, SETGID]",
		"ipv4_address: 172.29.53.3",
		"rootguard-adguard-auth:/etc/nginx/secrets:ro",
		"- /etc/nginx/conf.d",
		"rootguard-adguard-auth:\n    external: true",
	} {
		if !strings.Contains(enabled, expected) {
			t.Fatalf("expected enabled compose to contain %q:\n%s", expected, enabled)
		}
	}

	disabled, err := renderCompose(
		Config{DNSBindAddress: "192.168.1.2", DNSPort: 53, BlockpageEnabled: false},
		"rootguard-unbound:test",
		"adguard:test",
		"ghcr.io/foxly-it/rootguard-blockpage:latest",
		"172.29.53.0/24",
	)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(disabled, "blockpage") {
		t.Fatalf("expected disabled compose to omit the blockpage service:\n%s", disabled)
	}
}

func TestDeploymentPersistsCompletedState(t *testing.T) {
	dataDir := t.TempDir()
	var mu sync.Mutex
	var commands []string
	manager := NewManager(Options{
		DataDir:             dataDir,
		CoreContainer:       "rootguard-core",
		UnboundImage:        "rootguard-unbound:test",
		AdGuardImage:        "adguard:test",
		DNSNetworkCIDR:      "172.29.53.0/24",
		AttestationVerifier: noopAttestationVerifier,
		Run: func(_ context.Context, arguments ...string) ([]byte, error) {
			mu.Lock()
			commands = append(commands, strings.Join(arguments, " "))
			mu.Unlock()
			if arguments[0] == "inspect" {
				return []byte("healthy\n"), nil
			}
			return []byte("ok"), nil
		},
	})

	_, err := manager.Start(context.Background(), Config{
		DNSBindAddress: "192.168.1.2",
		DNSPort:        53,
	})
	if err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for manager.Status().State == StateDeploying && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if status := manager.Status(); status.State != StateInstalled {
		t.Fatalf("expected installed state, got %#v", status)
	}

	reloaded := NewManager(Options{DataDir: dataDir})
	if status := reloaded.Status(); status.State != StateInstalled {
		t.Fatalf("expected persisted installed state, got %#v", status)
	}

	mu.Lock()
	allCommands := strings.Join(commands, "\n")
	mu.Unlock()
	for _, expected := range []string{
		"compose version", "compose --project-name rootguard-dns", "inspect --format",
		// Pinned, not a bare "network connect" - an unpinned connect lets
		// Docker hand the controller whatever address happens to be free,
		// including unbound's or blockpage's own reserved slot if that
		// service is down at the moment the controller (re)connects.
		"network connect --ip 172.29.53.5 rootguard-dns",
	} {
		if !strings.Contains(allCommands, expected) {
			t.Fatalf("expected command containing %q in:\n%s", expected, allCommands)
		}
	}
	if strings.Contains(allCommands, "rootguard-blockpage") {
		t.Fatalf("expected no blockpage config reload when the blockpage is disabled:\n%s", allCommands)
	}
}

func TestReconcilePinsControllerNetworkAddress(t *testing.T) {
	dataDir := t.TempDir()
	var mu sync.Mutex
	var commands []string
	manager := NewManager(Options{
		DataDir:             dataDir,
		CoreContainer:       "rootguard-core",
		UnboundImage:        "rootguard-unbound:test",
		AdGuardImage:        "adguard:test",
		DNSNetworkCIDR:      "172.29.53.0/24",
		AttestationVerifier: noopAttestationVerifier,
		Run: func(_ context.Context, arguments ...string) ([]byte, error) {
			mu.Lock()
			commands = append(commands, strings.Join(arguments, " "))
			mu.Unlock()
			if arguments[0] == "inspect" {
				return []byte("healthy\n"), nil
			}
			return []byte("ok"), nil
		},
	})

	if _, err := manager.Start(context.Background(), Config{DNSBindAddress: "192.168.1.2", DNSPort: 53}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for manager.Status().State == StateDeploying && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	mu.Lock()
	commands = nil
	mu.Unlock()

	if err := manager.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	allCommands := strings.Join(commands, "\n")
	mu.Unlock()
	if !strings.Contains(allCommands, "network connect --ip 172.29.53.5 rootguard-dns rootguard-core") {
		t.Fatalf("expected Reconcile to reconnect with a pinned address, got:\n%s", allCommands)
	}
}

func TestDeploymentRestartsBlockpageAfterBootstrapWhenEnabled(t *testing.T) {
	dataDir := t.TempDir()
	var mu sync.Mutex
	var commands []string
	manager := NewManager(Options{
		DataDir:             dataDir,
		CoreContainer:       "rootguard-core",
		UnboundImage:        "rootguard-unbound:test",
		AdGuardImage:        "adguard:test",
		BlockpageImage:      "ghcr.io/foxly-it/rootguard-blockpage:latest",
		DNSNetworkCIDR:      "172.29.53.0/24",
		AttestationVerifier: noopAttestationVerifier,
		Run: func(_ context.Context, arguments ...string) ([]byte, error) {
			mu.Lock()
			commands = append(commands, strings.Join(arguments, " "))
			mu.Unlock()
			if arguments[0] == "inspect" {
				return []byte("healthy\n"), nil
			}
			return []byte("ok"), nil
		},
	})

	_, err := manager.Start(context.Background(), Config{
		DNSBindAddress:   "192.168.1.2",
		DNSPort:          53,
		BlockpageEnabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for manager.Status().State == StateDeploying && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if status := manager.Status(); status.State != StateInstalled {
		t.Fatalf("expected installed state, got %#v", status)
	}

	mu.Lock()
	allCommands := strings.Join(commands, "\n")
	mu.Unlock()
	for _, expected := range []string{
		"exec rootguard-blockpage sh /docker-entrypoint.d/19-render-blockpage-conf.sh",
		"exec rootguard-blockpage nginx -s reload",
	} {
		if !strings.Contains(allCommands, expected) {
			t.Fatalf("expected blockpage config to be re-rendered and reloaded after bootstrap so it picks up its AdGuard auth token, missing %q in:\n%s", expected, allCommands)
		}
	}
	if strings.Contains(allCommands, "restart rootguard-blockpage") {
		t.Fatalf("expected a config reload, not a container restart (races AdGuard's dynamic IP for blockpage's static one), got:\n%s", allCommands)
	}
}

func TestRenderedComposeRejectsInvalidInternalNetwork(t *testing.T) {
	_, err := renderCompose(
		Config{DNSBindAddress: "0.0.0.0", DNSPort: 53},
		"rootguard-unbound:test",
		"adguard:test",
		"rootguard-blockpage:test",
		"not-a-network",
	)
	if err == nil {
		t.Fatal("expected invalid internal network to be rejected")
	}
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
	dataDir := filepath.Join(blocker, "installation")

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

// TestStatusSurfacesPersistFailureAndSelfHeals is the regression test for a
// follow-up review finding: OnPersistError above made a failed write
// diagnosable from the container's own logs, but Status() itself still
// reported whatever the in-memory state was with no indication the on-disk
// record backing it might be stale. PersistError/PersistErrorAt must
// appear in Status() the moment a persist fails, and disappear again the
// moment a later persist succeeds - self-healing once the underlying
// disk/permissions problem does, without any caller needing to notice or
// retry anything itself.
func TestStatusSurfacesPersistFailureAndSelfHeals(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blocker, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	dataDir := filepath.Join(blocker, "installation")

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

	// Repoint at a real, writable directory - the same recovery an
	// operator fixing a full disk or a permissions problem would perform -
	// and confirm the very next successful persist clears both fields.
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

// TestDeployRefusesActivationWhenAttestationFails is the regression test
// for a review finding: RequireAttestation was wired into both updater
// packages, but never into this one, so a fresh install's very first
// Unbound activation - often the only deployment event most
// installations ever have - skipped the check entirely. Uses a fake
// verifier rather than a real cosign binary; DiscoverHosts-style unit
// tests can't exercise the real cosign path anyway.
func TestDeployRefusesActivationWhenAttestationFails(t *testing.T) {
	dataDir := t.TempDir()
	manager := NewManager(Options{
		DataDir:        dataDir,
		CoreContainer:  "rootguard-core",
		UnboundImage:   "rootguard-unbound:test",
		AdGuardImage:   "adguard:test",
		DNSNetworkCIDR: "172.29.53.0/24",
		AttestationVerifier: func(_ context.Context, service, _ string) error {
			return fmt.Errorf("attestation for %s is missing", service)
		},
		Run: func(_ context.Context, arguments ...string) ([]byte, error) {
			if arguments[0] == "inspect" {
				return []byte("healthy\n"), nil
			}
			return []byte("ok"), nil
		},
	})

	if _, err := manager.Start(context.Background(), Config{DNSBindAddress: "192.168.1.2", DNSPort: 53}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for manager.Status().State == StateDeploying && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	status := manager.Status()
	if status.State != StateFailed {
		t.Fatalf("expected the deployment to fail closed on a failed attestation, got %#v", status)
	}
	if !strings.Contains(status.Error, "attestation") {
		t.Fatalf("expected the failure to mention attestation, got %q", status.Error)
	}
}

// TestRestoreRefusesActivationWhenAttestationFails is
// TestDeployRefusesActivationWhenAttestationFails's counterpart for the
// separate restoreDeploy code path (Restore), which has its own point of
// no return before compose up.
func TestRestoreRefusesActivationWhenAttestationFails(t *testing.T) {
	dataDir := t.TempDir()
	manager := NewManager(Options{
		DataDir:        dataDir,
		CoreContainer:  "rootguard-core",
		UnboundImage:   "rootguard-unbound:test",
		AdGuardImage:   "adguard:test",
		DNSNetworkCIDR: "172.29.53.0/24",
		AttestationVerifier: func(_ context.Context, service, _ string) error {
			return fmt.Errorf("attestation for %s is missing", service)
		},
		Run: func(_ context.Context, arguments ...string) ([]byte, error) {
			// RestorePreflight's own per-resource existence check
			// ("container inspect ...", "volume inspect ...", "network
			// inspect ...") needs to report every managed resource as
			// absent for Restore to even consider this a clean target -
			// unrelated to the attestation gate this test is actually
			// about, but a precondition Restore checks first.
			if len(arguments) >= 2 && arguments[1] == "inspect" {
				return []byte("no such resource"), errors.New("exit status 1")
			}
			return []byte("ok"), nil
		},
	})

	_, err := manager.Restore(context.Background(), Config{DNSBindAddress: "192.168.1.2", DNSPort: 53},
		func(context.Context) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "attestation") {
		t.Fatalf("expected Restore to fail closed on a failed attestation, got %v", err)
	}
}
