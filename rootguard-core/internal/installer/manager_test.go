package installer

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

// TestRunComposeUpRetriesTransientPortBindConflict covers the race a static
// `docker ps` preflight check cannot rule out: a container that just
// stopped can hold its published port for a moment after it's gone from
// `docker ps`, so the very next `up -d` on that port needs a short retry
// instead of failing the whole deployment outright.
func TestRunComposeUpRetriesTransientPortBindConflict(t *testing.T) {
	var attempts int
	manager := NewManager(Options{
		DataDir:                t.TempDir(),
		ComposeUpRetryAttempts: 3,
		ComposeUpRetryDelay:    time.Millisecond,
		Run: func(_ context.Context, _ ...string) ([]byte, error) {
			attempts++
			if attempts < 3 {
				return nil, errors.New("Bind for 0.0.0.0:53 failed: port is already allocated")
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
		DataDir:        dataDir,
		CoreContainer:  "rootguard-core",
		UnboundImage:   "rootguard-unbound:test",
		AdGuardImage:   "adguard:test",
		DNSNetworkCIDR: "172.29.53.0/24",
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
		DataDir:        dataDir,
		CoreContainer:  "rootguard-core",
		UnboundImage:   "rootguard-unbound:test",
		AdGuardImage:   "adguard:test",
		DNSNetworkCIDR: "172.29.53.0/24",
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
		DataDir:        dataDir,
		CoreContainer:  "rootguard-core",
		UnboundImage:   "rootguard-unbound:test",
		AdGuardImage:   "adguard:test",
		BlockpageImage: "ghcr.io/foxly-it/rootguard-blockpage:latest",
		DNSNetworkCIDR: "172.29.53.0/24",
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
