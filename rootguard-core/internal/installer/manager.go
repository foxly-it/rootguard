package installer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	StateNotInstalled = "not_installed"
	StateDeploying    = "deploying"
	StateInstalled    = "installed"
	StateFailed       = "failed"
)

var (
	ErrInvalidConfig = errors.New("invalid installation configuration")
	ErrDeploying     = errors.New("installation is already running")
	ErrNotClean      = errors.New("restore requires a clean RootGuard installation")
)

type Config struct {
	DNSBindAddress   string `json:"dns_bind_address"`
	DNSPort          int    `json:"dns_port"`
	AdGuardChannel   string `json:"adguard_channel"`
	BlockpageEnabled bool   `json:"blockpage_enabled"`
}

type Check struct {
	ID      string `json:"id"`
	Code    string `json:"code"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
	Action  string `json:"action,omitempty"`
}

type Preflight struct {
	Ready  bool    `json:"ready"`
	Config Config  `json:"config"`
	Checks []Check `json:"checks"`
}

type Step struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type Diagnostic struct {
	Code      string `json:"code"`
	Phase     string `json:"phase"`
	Message   string `json:"message"`
	Detail    string `json:"detail,omitempty"`
	Action    string `json:"action"`
	Retryable bool   `json:"retryable"`
}

type Status struct {
	State      string      `json:"state"`
	Config     *Config     `json:"config,omitempty"`
	Steps      []Step      `json:"steps"`
	Error      string      `json:"error,omitempty"`
	Diagnostic *Diagnostic `json:"diagnostic,omitempty"`
	UpdatedAt  time.Time   `json:"updated_at"`
}

type CommandRunner func(context.Context, ...string) ([]byte, error)
type BootstrapFunc func(context.Context, string) error
type RestoreFunc func(context.Context) error

type Options struct {
	DataDir          string
	CoreContainer    string
	UnboundImage     string
	AdGuardImage     string
	AdGuardBetaImage string
	BlockpageImage   string
	DNSNetworkCIDR   string
	Run              CommandRunner
	Bootstrap        BootstrapFunc
	// ComposeUpRetryAttempts/ComposeUpRetryDelay bound retries of a
	// transient port-bind race on `compose up` (see runComposeUp). Default
	// to 3 attempts / 2s when unset; tests may override for faster runs.
	ComposeUpRetryAttempts int
	ComposeUpRetryDelay    time.Duration
}

type Manager struct {
	mu                     sync.RWMutex
	status                 Status
	dataDir                string
	coreContainer          string
	unboundImage           string
	adGuardImage           string
	adGuardBetaImage       string
	blockpageImage         string
	dnsNetworkCIDR         string
	run                    CommandRunner
	bootstrap              BootstrapFunc
	composeUpRetryAttempts int
	composeUpRetryDelay    time.Duration
}

func NewManager(options Options) *Manager {
	if options.Run == nil {
		options.Run = runDocker
	}
	if options.Bootstrap == nil {
		options.Bootstrap = func(context.Context, string) error { return nil }
	}
	if options.ComposeUpRetryAttempts <= 0 {
		options.ComposeUpRetryAttempts = 3
	}
	if options.ComposeUpRetryDelay <= 0 {
		options.ComposeUpRetryDelay = 2 * time.Second
	}
	manager := &Manager{
		dataDir:                options.DataDir,
		coreContainer:          options.CoreContainer,
		unboundImage:           options.UnboundImage,
		adGuardImage:           options.AdGuardImage,
		adGuardBetaImage:       options.AdGuardBetaImage,
		blockpageImage:         options.BlockpageImage,
		dnsNetworkCIDR:         options.DNSNetworkCIDR,
		run:                    options.Run,
		bootstrap:              options.Bootstrap,
		composeUpRetryAttempts: options.ComposeUpRetryAttempts,
		composeUpRetryDelay:    options.ComposeUpRetryDelay,
		status: Status{
			State:     StateNotInstalled,
			Steps:     []Step{},
			UpdatedAt: time.Now().UTC(),
		},
	}
	manager.load()
	if manager.status.State == StateDeploying {
		manager.status.State = StateFailed
		manager.setDiagnosticLocked(Diagnostic{
			Code: "deployment_interrupted", Phase: "recovery",
			Message:   "The previous deployment was interrupted.",
			Action:    "Review the completed steps and start the deployment again. RootGuard reuses its persisted configuration safely.",
			Retryable: true,
		})
		manager.status.UpdatedAt = time.Now().UTC()
		_ = manager.persist()
	}
	return manager
}

func (m *Manager) Status() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return cloneStatus(m.status)
}

// Reconcile reconnects the long-lived controller after it was recreated by an
// update. Docker preserves a manual network attachment on restart, but not when
// Compose replaces the controller container.
func (m *Manager) Reconcile(ctx context.Context) error {
	status := m.Status()
	if status.State != StateInstalled {
		return nil
	}
	address, err := coreAddress(m.dnsNetworkCIDR)
	if err != nil {
		return err
	}
	output, err := m.run(ctx, "network", "connect", "--ip", address, "rootguard-dns", m.coreContainer)
	if err == nil || strings.Contains(strings.ToLower(string(output)), "already exists") {
		return nil
	}
	return fmt.Errorf("reconnect RootGuard controller to DNS network: %w: %s", err, strings.TrimSpace(string(output)))
}

func (m *Manager) Preflight(ctx context.Context, config Config) Preflight {
	config = normalizeConfig(config)
	checks := validateConfig(config)

	dockerOK := true
	if _, err := m.run(ctx, "version", "--format", "{{.Server.Version}}"); err != nil {
		dockerOK = false
		checks = append(checks, Check{
			ID: "docker", Code: "docker_unreachable", OK: false,
			Message: "Docker Engine is not reachable through the RootGuard controller.",
			Action:  "Verify the Docker socket mount and Docker Engine status.",
		})
	} else {
		checks = append(checks, Check{
			ID: "docker", Code: "docker_reachable", OK: true,
			Message: "Docker Engine is reachable.",
		})
	}

	if _, err := m.run(ctx, "compose", "version", "--short"); err != nil {
		checks = append(checks, Check{
			ID: "compose", Code: "compose_unavailable", OK: false,
			Message: "The Docker Compose plugin is not available in the controller.",
			Action:  "Use the supported RootGuard Core image or install the Docker Compose plugin.",
		})
	} else {
		checks = append(checks, Check{
			ID: "compose", Code: "compose_available", OK: true,
			Message: "Docker Compose is available.",
		})
	}

	if dockerOK && validNetworkConfig(config) {
		output, err := m.run(ctx, "ps", "--format", "{{.Names}}|{{.Ports}}")
		if err != nil {
			checks = append(checks, Check{
				ID: "dns_port_available", Code: "port_check_failed", OK: false,
				Message: "Docker port assignments could not be inspected.",
				Action:  "Verify Docker access and run the preflight again.",
			})
		} else if owner := occupiedDockerPort(string(output), config.DNSBindAddress, config.DNSPort); owner != "" {
			checks = append(checks, Check{
				ID: "dns_port_available", Code: "dns_port_occupied", OK: false,
				Message: fmt.Sprintf("DNS port %s:%d is already published.", config.DNSBindAddress, config.DNSPort),
				Detail:  owner,
				Action:  "Stop or reconfigure the conflicting DNS service, then run the preflight again.",
			})
		} else if busy, detail := m.probeHostPortBusy(ctx, config.DNSBindAddress, config.DNSPort); busy {
			checks = append(checks, Check{
				ID: "dns_port_available", Code: "dns_port_occupied", OK: false,
				Message: fmt.Sprintf("DNS port %s:%d is already in use on this host.", config.DNSBindAddress, config.DNSPort),
				Detail:  detail,
				Action:  "Stop or reconfigure the conflicting service - a non-Docker process such as systemd-resolved is a common cause - then run the preflight again.",
			})
		} else {
			checks = append(checks, Check{
				ID: "dns_port_available", Code: "dns_port_available", OK: true,
				Message: "No conflicting port publication was found.",
			})
		}
	}

	ready := true
	for _, check := range checks {
		if !check.OK {
			ready = false
		}
	}
	return Preflight{Ready: ready, Config: config, Checks: checks}
}

func (m *Manager) Start(ctx context.Context, config Config) (Status, error) {
	report := m.Preflight(ctx, config)
	if !report.Ready {
		return m.Status(), ErrInvalidConfig
	}

	m.mu.Lock()
	if m.status.State == StateDeploying {
		m.mu.Unlock()
		return Status{}, ErrDeploying
	}
	m.status = Status{
		State:  StateDeploying,
		Config: &report.Config,
		Steps: []Step{
			{ID: "prepare", Status: "pending", Message: "Preparing the managed stack"},
			{ID: "pull", Status: "pending", Message: "Downloading configured service images"},
			{ID: "start", Status: "pending", Message: "Starting Unbound and AdGuard Home"},
			{ID: "connect", Status: "pending", Message: "Connecting the controller"},
			{ID: "bootstrap", Status: "pending", Message: "Configuring the protected DNS chain"},
		},
		UpdatedAt: time.Now().UTC(),
	}
	if err := m.persistLocked(); err != nil {
		m.status.State = StateFailed
		m.setDiagnosticLocked(classifyDeploymentError("prepare", fmt.Errorf("persist installation state: %w", err)))
		m.status.UpdatedAt = time.Now().UTC()
		status := cloneStatus(m.status)
		m.mu.Unlock()
		return status, fmt.Errorf("persist installation state: %w", err)
	}
	status := cloneStatus(m.status)
	m.mu.Unlock()

	go m.deploy(report.Config)
	return status, nil
}

// Restore deploys the managed stack synchronously and invokes restoreData
// after fresh containers and volumes have been created but before any service
// is started.  It is intentionally restricted to a never-installed target.
func (m *Manager) Restore(ctx context.Context, config Config, restoreData RestoreFunc) (Status, error) {
	report := m.RestorePreflight(ctx, config)
	if !report.Ready {
		return m.Status(), ErrNotClean
	}
	m.mu.Lock()
	if m.status.State != StateNotInstalled && m.status.State != StateFailed {
		m.mu.Unlock()
		return Status{}, ErrNotClean
	}
	m.status = Status{State: StateDeploying, Config: &report.Config, Steps: []Step{
		{ID: "prepare", Status: "pending", Message: "Preparing the managed stack"},
		{ID: "pull", Status: "pending", Message: "Downloading configured service images"},
		{ID: "create", Status: "pending", Message: "Creating empty service volumes"},
		{ID: "restore", Status: "pending", Message: "Restoring verified backup data"},
		{ID: "start", Status: "pending", Message: "Starting restored services"},
		{ID: "connect", Status: "pending", Message: "Connecting the controller"},
		{ID: "bootstrap", Status: "pending", Message: "Verifying the protected DNS chain"},
	}, UpdatedAt: time.Now().UTC()}
	if err := m.persistLocked(); err != nil {
		m.mu.Unlock()
		return Status{}, err
	}
	m.mu.Unlock()
	if err := m.restoreDeploy(ctx, report.Config, restoreData); err != nil {
		return m.Status(), err
	}
	return m.Status(), nil
}

func (m *Manager) RestorePreflight(ctx context.Context, config Config) Preflight {
	report := m.Preflight(ctx, config)
	status := m.Status()
	clean := status.State == StateNotInstalled || status.State == StateFailed
	report.Checks = append(report.Checks, Check{ID: "clean_installation", Code: "clean_installation_required", OK: clean,
		Message: "RootGuard has not deployed a managed DNS stack on this installation.", Action: "Use a new RootGuard installation for full restore."})
	for _, resource := range []struct{ kind, name string }{{"container", "rootguard-unbound"}, {"container", "rootguard-adguard"}, {"container", "rootguard-blockpage"}, {"volume", "rootguard-unbound-state"}, {"volume", "rootguard-adguard-work"}, {"volume", "rootguard-adguard-config"}, {"network", "rootguard-dns"}} {
		output, err := m.run(ctx, resource.kind, "inspect", resource.name)
		detail := strings.ToLower(string(output))
		missing := err != nil && (strings.Contains(detail, "no such") || strings.Contains(detail, "not found"))
		report.Checks = append(report.Checks, Check{ID: "restore_" + resource.kind + "_" + resource.name, Code: "restore_target_resource_absent", OK: missing,
			Message: fmt.Sprintf("Managed Docker %s %s is absent.", resource.kind, resource.name), Action: "Remove stale managed resources only after confirming this is a clean replacement host."})
		clean = clean && missing
	}
	report.Ready = report.Ready && clean
	return report
}

func (m *Manager) restoreDeploy(parent context.Context, config Config, restoreData RestoreFunc) error {
	ctx, cancel := context.WithTimeout(parent, 10*time.Minute)
	defer cancel()
	composePath := ""
	created := false
	fail := func(phase string, err error) error {
		if created {
			_, _ = m.run(ctx, "network", "disconnect", "-f", "rootguard-dns", m.coreContainer)
			_, _ = m.run(ctx, "compose", "--project-name", "rootguard-dns", "-f", composePath, "down", "--volumes", "--remove-orphans")
		}
		m.fail(phase, err)
		return err
	}
	_ = m.setStep("prepare", "running", "Writing the versioned RootGuard stack definition")
	var err error
	composePath, err = m.writeCompose(config)
	if err != nil {
		return fail("prepare", err)
	}
	_ = m.setStep("prepare", "done", "Managed stack definition is ready")
	_ = m.setStep("pull", "running", "Downloading configured service images")
	if _, err = m.run(ctx, "compose", "--project-name", "rootguard-dns", "-f", composePath, "pull"); err != nil {
		return fail("pull", err)
	}
	_ = m.setStep("pull", "done", "Service images are available")
	_ = m.setStep("create", "running", "Creating stopped containers and empty service volumes")
	if _, err = m.run(ctx, "compose", "--project-name", "rootguard-dns", "-f", composePath, "create"); err != nil {
		return fail("create", err)
	}
	created = true
	_ = m.setStep("create", "done", "Stopped containers and service volumes are ready")
	_ = m.setStep("restore", "running", "Copying verified backup data into the clean installation")
	if err = restoreData(ctx); err != nil {
		return fail("restore", err)
	}
	_ = m.setStep("restore", "done", "Verified backup data is restored")
	_ = m.setStep("start", "running", "Starting restored Unbound and AdGuard Home")
	if _, err = m.runComposeUp(ctx, "compose", "--project-name", "rootguard-dns", "-f", composePath, "up", "-d"); err != nil {
		return fail("start", err)
	}
	_ = m.setStep("start", "done", "Restored DNS containers are running")
	_ = m.setStep("connect", "running", "Connecting the controller to the private DNS network")
	coreIP, err := coreAddress(m.dnsNetworkCIDR)
	if err != nil {
		return fail("connect", err)
	}
	if output, runErr := m.run(ctx, "network", "connect", "--ip", coreIP, "rootguard-dns", m.coreContainer); runErr != nil && !strings.Contains(strings.ToLower(string(output)), "already exists") {
		return fail("connect", runErr)
	}
	_ = m.setStep("connect", "done", "Controller is connected to the private DNS network")
	_ = m.setStep("bootstrap", "running", "Waiting for Unbound and verifying restored AdGuard Home")
	if err = m.waitForUnbound(ctx); err != nil {
		return fail("bootstrap", err)
	}
	blockPageIP := ""
	if config.BlockpageEnabled {
		blockPageIP = config.DNSBindAddress
	}
	if err = m.bootstrap(ctx, blockPageIP); err != nil {
		return fail("bootstrap", err)
	}
	if config.BlockpageEnabled {
		if _, err = m.run(ctx, "exec", "rootguard-blockpage", "sh", "/docker-entrypoint.d/19-render-blockpage-conf.sh"); err != nil {
			return fail("bootstrap", err)
		}
		if _, err = m.run(ctx, "exec", "rootguard-blockpage", "nginx", "-s", "reload"); err != nil {
			return fail("bootstrap", err)
		}
	}
	_ = m.setStep("bootstrap", "done", "Restored DNS chain is healthy")
	m.mu.Lock()
	m.status.State, m.status.Error, m.status.Diagnostic, m.status.UpdatedAt = StateInstalled, "", nil, time.Now().UTC()
	err = m.persistLocked()
	m.mu.Unlock()
	return err
}

func (m *Manager) deploy(config Config) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	if err := m.setStep("prepare", "running", "Writing the versioned RootGuard stack definition"); err != nil {
		m.fail("prepare", err)
		return
	}
	composePath, err := m.writeCompose(config)
	if err != nil {
		m.fail("prepare", err)
		return
	}
	_ = m.setStep("prepare", "done", "Managed stack definition is ready")

	_ = m.setStep("pull", "running", "Downloading configured service images")
	if _, err := m.run(ctx, "compose", "--project-name", "rootguard-dns", "-f", composePath, "pull"); err != nil {
		m.fail("pull", fmt.Errorf("pull RootGuard service images: %w", err))
		return
	}
	_ = m.setStep("pull", "done", "Service images are available")

	_ = m.setStep("start", "running", "Starting Unbound and AdGuard Home")
	if _, err := m.runComposeUp(ctx, "compose", "--project-name", "rootguard-dns", "-f", composePath, "up", "-d"); err != nil {
		m.fail("start", fmt.Errorf("start RootGuard DNS stack: %w", err))
		return
	}
	_ = m.setStep("start", "done", "DNS containers were created")

	_ = m.setStep("connect", "running", "Connecting the controller to the private DNS network")
	coreIP, err := coreAddress(m.dnsNetworkCIDR)
	if err != nil {
		m.fail("connect", err)
		return
	}
	if output, err := m.run(ctx, "network", "connect", "--ip", coreIP, "rootguard-dns", m.coreContainer); err != nil &&
		!strings.Contains(strings.ToLower(string(output)), "already exists") {
		m.fail("connect", fmt.Errorf("connect RootGuard controller to DNS network: %w: %s", err, strings.TrimSpace(string(output))))
		return
	}
	_ = m.setStep("connect", "done", "Controller is connected to the private DNS network")

	_ = m.setStep("bootstrap", "running", "Waiting for Unbound and securing AdGuard Home")
	if err := m.waitForUnbound(ctx); err != nil {
		m.fail("bootstrap", err)
		return
	}
	blockPageIP := ""
	if config.BlockpageEnabled {
		blockPageIP = config.DNSBindAddress
	}
	if err := m.bootstrap(ctx, blockPageIP); err != nil {
		m.fail("bootstrap", fmt.Errorf("bootstrap AdGuard Home: %w", err))
		return
	}
	if config.BlockpageEnabled {
		// Re-render blockpage's nginx config (now that Core has published its
		// AdGuard auth token) and reload nginx in place, rather than
		// restarting the container: a restart tears down and re-creates its
		// network endpoint, racing AdGuard's dynamically-assigned address for
		// the static IP blockpage needs - a reload has no such networking
		// side effect and keeps the blockpage continuously reachable.
		if _, err := m.run(ctx, "exec", "rootguard-blockpage", "sh", "/docker-entrypoint.d/19-render-blockpage-conf.sh"); err != nil {
			m.fail("bootstrap", fmt.Errorf("render blockpage nginx config with its AdGuard auth token: %w", err))
			return
		}
		if _, err := m.run(ctx, "exec", "rootguard-blockpage", "nginx", "-s", "reload"); err != nil {
			m.fail("bootstrap", fmt.Errorf("reload blockpage nginx to pick up its AdGuard auth token: %w", err))
			return
		}
	}
	_ = m.setStep("bootstrap", "done", "AdGuard Home forwards exclusively to Unbound")

	m.mu.Lock()
	m.status.State = StateInstalled
	m.status.Error = ""
	m.status.Diagnostic = nil
	m.status.UpdatedAt = time.Now().UTC()
	_ = m.persistLocked()
	m.mu.Unlock()
}

func (m *Manager) waitForUnbound(ctx context.Context) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		output, err := m.run(ctx, "inspect", "--format", "{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}", "rootguard-unbound")
		if err == nil && strings.TrimSpace(string(output)) == "healthy" {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for Unbound health: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func (m *Manager) writeCompose(config Config) (string, error) {
	if err := os.MkdirAll(m.dataDir, 0700); err != nil {
		return "", fmt.Errorf("create installation data directory: %w", err)
	}
	adGuardImage := m.adGuardImage
	if config.AdGuardChannel == "beta" {
		adGuardImage = m.adGuardBetaImage
	}
	content, err := renderCompose(config, m.unboundImage, adGuardImage, m.blockpageImage, m.dnsNetworkCIDR)
	if err != nil {
		return "", err
	}
	path := filepath.Join(m.dataDir, "compose.yaml")
	temp := path + ".tmp"
	if err := os.WriteFile(temp, []byte(content), 0600); err != nil {
		return "", fmt.Errorf("write temporary stack definition: %w", err)
	}
	if err := os.Rename(temp, path); err != nil {
		return "", fmt.Errorf("activate stack definition: %w", err)
	}
	return path, nil
}

func renderCompose(config Config, unboundImage, adGuardImage, blockpageImage, networkCIDR string) (string, error) {
	resolverAddress, err := resolverAddress(networkCIDR)
	if err != nil {
		return "", err
	}
	adGuardAddress, err := adguardAddress(networkCIDR)
	if err != nil {
		return "", err
	}
	var blockpageService string
	if config.BlockpageEnabled {
		blockpageIP, err := blockpageAddress(networkCIDR)
		if err != nil {
			return "", err
		}
		blockpageService = fmt.Sprintf(`
  blockpage:
    image: %s
    container_name: rootguard-blockpage
    restart: unless-stopped
    read_only: true
    tmpfs:
      - /var/cache/nginx
      - /run
      - /etc/nginx/conf.d
    volumes:
      - rootguard-adguard-auth:/etc/nginx/secrets:ro
    cap_drop: [ALL]
    # nginx's master process starts as root and drops worker processes -
    # the ones that actually handle client connections - to the non-root
    # nginx user. That handoff needs exactly these three capabilities;
    # everything else stays dropped.
    cap_add: [CHOWN, SETUID, SETGID]
    security_opt:
      - no-new-privileges:true
    labels:
      io.rootguard.managed: "true"
      io.rootguard.component: "blockpage"
    ports:
      - "%s:80:8080/tcp"
    networks:
      dns:
        ipv4_address: %s
`, blockpageImage, config.DNSBindAddress, blockpageIP)
	}
	return fmt.Sprintf(`name: rootguard-dns

services:
  unbound:
    image: %s
    container_name: rootguard-unbound
    restart: unless-stopped
    read_only: true
    cap_drop: [ALL]
    security_opt:
      - no-new-privileges:true
    labels:
      io.rootguard.managed: "true"
      io.rootguard.component: "unbound"
    volumes:
      - rootguard-unbound-config:/etc/unbound/unbound.d
      - rootguard-unbound-state:/var/lib/unbound
    networks:
      dns:
        ipv4_address: %s

  adguard:
    image: %s
    container_name: rootguard-adguard
    restart: unless-stopped
    depends_on:
      unbound:
        condition: service_healthy
    labels:
      io.rootguard.managed: "true"
      io.rootguard.component: "adguard"
    ports:
      - "%s:%d:53/tcp"
      - "%s:%d:53/udp"
    volumes:
      - rootguard-adguard-work:/opt/adguardhome/work
      - rootguard-adguard-config:/opt/adguardhome/conf
    networks:
      dns:
        ipv4_address: %s
%s
networks:
  dns:
    name: rootguard-dns
    ipam:
      config:
        - subnet: %s

volumes:
  rootguard-unbound-config:
    external: true
  rootguard-adguard-auth:
    external: true
  rootguard-unbound-state:
    name: rootguard-unbound-state
  rootguard-adguard-work:
    name: rootguard-adguard-work
  rootguard-adguard-config:
    name: rootguard-adguard-config
`, unboundImage, resolverAddress, adGuardImage, config.DNSBindAddress, config.DNSPort,
		config.DNSBindAddress, config.DNSPort, adGuardAddress, blockpageService, networkCIDR), nil
}

func resolverAddress(networkCIDR string) (string, error) {
	return networkAddress(networkCIDR, 2)
}

func blockpageAddress(networkCIDR string) (string, error) {
	return networkAddress(networkCIDR, 3)
}

func adguardAddress(networkCIDR string) (string, error) {
	return networkAddress(networkCIDR, 4)
}

// coreAddress is the controller's own pinned address on the DNS network -
// it isn't a compose-managed service (it joins via a plain "docker network
// connect" in Reconcile/Deploy below, not a networks: block in the
// rendered compose file), but it still needs a fixed slot outside the ones
// reserved for unbound/blockpage/adguard: an unpinned connect lets Docker
// hand out whatever address is free at that moment, including one of
// those three services' own reserved addresses if that service happens to
// be down when the controller (re)connects - permanently blocking that
// service from reclaiming its own static IP afterwards.
func coreAddress(networkCIDR string) (string, error) {
	return networkAddress(networkCIDR, 5)
}

func networkAddress(networkCIDR string, offset byte) (string, error) {
	ip, network, err := net.ParseCIDR(networkCIDR)
	if err != nil || ip.To4() == nil {
		return "", fmt.Errorf("invalid IPv4 DNS network %q", networkCIDR)
	}
	address := append(net.IP(nil), ip.To4()...)
	address[3] += offset
	if !network.Contains(address) {
		return "", fmt.Errorf("DNS network %q has no address at offset %d", networkCIDR, offset)
	}
	return address.String(), nil
}

func normalizeConfig(config Config) Config {
	config.DNSBindAddress = strings.TrimSpace(config.DNSBindAddress)
	config.AdGuardChannel = strings.ToLower(strings.TrimSpace(config.AdGuardChannel))
	if config.AdGuardChannel == "" {
		config.AdGuardChannel = "stable"
	}
	return config
}

func validateConfig(config Config) []Check {
	var checks []Check
	if config.AdGuardChannel != "stable" && config.AdGuardChannel != "beta" {
		checks = append(checks, Check{
			ID: "adguard_channel", Code: "invalid_adguard_channel", OK: false,
			Message: "Choose the Stable or Beta AdGuard Home release channel.",
			Action:  "Select Stable for normal operation or Beta only for intentional testing.",
		})
	} else {
		checks = append(checks, Check{
			ID: "adguard_channel", Code: "adguard_channel_valid", OK: true,
			Message: fmt.Sprintf("AdGuard Home release channel %s is selected.", config.AdGuardChannel),
		})
	}
	ip := net.ParseIP(config.DNSBindAddress)
	if ip == nil || ip.To4() == nil {
		checks = append(checks, Check{
			ID: "dns_address", Code: "invalid_dns_address", OK: false,
			Message: "Enter an IPv4 address already assigned to the Docker host, or 0.0.0.0 for all addresses.",
			Action:  "Enter a valid IPv4 address such as 192.168.1.10 or use 0.0.0.0.",
		})
	} else {
		checks = append(checks, Check{
			ID: "dns_address", Code: "dns_address_valid", OK: true,
			Message: "The DNS bind address has a valid IPv4 format.",
		})
	}
	if config.DNSPort < 1 || config.DNSPort > 65535 {
		checks = append(checks, Check{
			ID: "dns_port", Code: "invalid_dns_port", OK: false,
			Message: "The DNS port must be between 1 and 65535.",
			Action:  "Choose a port between 1 and 65535. Port 53 is recommended.",
		})
	} else {
		checks = append(checks, Check{
			ID: "dns_port", Code: "dns_port_valid", OK: true,
			Message: "The DNS port is valid. Docker performs the final host availability check during deployment.",
		})
	}
	if config.BlockpageEnabled {
		checks = append(checks, Check{
			ID: "blockpage", Code: "blockpage_enabled", OK: true,
			Message: "AdGuard Home will point blocked requests at the RootGuard blockpage.",
		})
	} else {
		checks = append(checks, Check{
			ID: "blockpage", Code: "blockpage_disabled", OK: true,
			Message: "The RootGuard blockpage is disabled; AdGuard Home keeps its default blocking response.",
		})
	}
	return checks
}

func (m *Manager) setStep(id, status, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for index := range m.status.Steps {
		if m.status.Steps[index].ID == id {
			m.status.Steps[index].Status = status
			m.status.Steps[index].Message = message
			m.status.UpdatedAt = time.Now().UTC()
			return m.persistLocked()
		}
	}
	return fmt.Errorf("unknown installation step %q", id)
}

func (m *Manager) fail(phase string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status.State = StateFailed
	m.setDiagnosticLocked(classifyDeploymentError(phase, err))
	m.status.UpdatedAt = time.Now().UTC()
	for index := range m.status.Steps {
		if m.status.Steps[index].Status == "running" {
			m.status.Steps[index].Status = "failed"
		}
	}
	_ = m.persistLocked()
}

func (m *Manager) setDiagnosticLocked(diagnostic Diagnostic) {
	m.status.Diagnostic = &diagnostic
	m.status.Error = diagnostic.Message
}

func classifyDeploymentError(phase string, err error) Diagnostic {
	detail := err.Error()
	lower := strings.ToLower(detail)
	diagnostic := Diagnostic{
		Code: "deployment_failed", Phase: phase, Message: "The DNS stack could not be deployed.",
		Detail: detail, Action: "Review the failed step and retry after correcting the cause.", Retryable: true,
	}
	switch {
	case phase == "pull":
		diagnostic.Code = "image_pull_failed"
		diagnostic.Message = "A configured service image could not be downloaded."
		diagnostic.Action = "Check registry access, the configured immutable image reference, and available disk space."
	case strings.Contains(lower, "cannot assign requested address"):
		diagnostic.Code = "host_address_unavailable"
		diagnostic.Message = "The selected DNS address is not available on the Docker host."
		diagnostic.Action = "Choose an IPv4 address assigned to this host or use 0.0.0.0, then retry."
	case isPortBindConflict(lower):
		diagnostic.Code = "dns_port_occupied"
		diagnostic.Message = "The selected DNS port is already in use."
		diagnostic.Action = "Stop or reconfigure the conflicting DNS service, then retry the deployment."
	case phase == "prepare":
		diagnostic.Code = "state_write_failed"
		diagnostic.Message = "RootGuard could not persist the installation state."
		diagnostic.Action = "Check permissions and free space of the rootguard-data volume before retrying."
	case phase == "connect":
		diagnostic.Code = "network_connect_failed"
		diagnostic.Message = "Core could not connect to the private DNS network."
		diagnostic.Action = "Inspect the Docker network state and retry the deployment."
	case phase == "bootstrap":
		diagnostic.Code = "dns_bootstrap_failed"
		diagnostic.Message = "The protected AdGuard-to-Unbound DNS chain did not become healthy."
		diagnostic.Action = "Review the AdGuard and Unbound diagnostics, then retry."
	}
	return diagnostic
}

func validNetworkConfig(config Config) bool {
	ip := net.ParseIP(config.DNSBindAddress)
	return ip != nil && ip.To4() != nil && config.DNSPort >= 1 && config.DNSPort <= 65535
}

func isPortBindConflict(lowerErr string) bool {
	return strings.Contains(lowerErr, "port is already allocated") || strings.Contains(lowerErr, "address already in use") ||
		(strings.Contains(lowerErr, "bind") && strings.Contains(lowerErr, "port"))
}

// runComposeUp absorbs a transient port-bind race that no preflight check
// can rule out ahead of time: a container that just stopped can leave its
// published port's kernel socket (or a lingering docker-proxy process) held
// for a moment after `docker ps` - and even Preflight's own docker-ps-based
// port check - already show it as gone, so the very next `up -d` for the
// same port can still lose the bind.
func (m *Manager) runComposeUp(ctx context.Context, args ...string) ([]byte, error) {
	var output []byte
	var err error
	for attempt := 1; attempt <= m.composeUpRetryAttempts; attempt++ {
		output, err = m.run(ctx, args...)
		if err == nil {
			return output, nil
		}
		// The CommandRunner contract returns output and err separately;
		// the production runner happens to fold output into err.Error(),
		// but an injected one isn't obligated to, so match against both
		// rather than assuming err.Error() alone carries the detail.
		detail := strings.ToLower(err.Error() + "\n" + string(output))
		if attempt == m.composeUpRetryAttempts || !isPortBindConflict(detail) {
			return output, err
		}
		select {
		case <-ctx.Done():
			return output, ctx.Err()
		case <-time.After(m.composeUpRetryDelay):
		}
	}
	return output, err
}

// probeHostPortBusy runs a throwaway container that actually publishes the
// requested address/port - the same mechanism `compose up` uses - to catch
// conflicts a `docker ps` scan can't see: a non-Docker process (most
// commonly systemd-resolved's stub listener on Debian/Ubuntu hosts) holds
// the port without ever appearing as a Docker container. Reuses Core's own
// already-running image via its container's image ID, so the probe needs
// no extra image pull; the container's entrypoint is overridden to exit
// immediately since only the publish attempt itself matters, and Docker
// performs that host-side bind at container start regardless of whether
// anything inside the container ever uses the port.
func (m *Manager) probeHostPortBusy(ctx context.Context, address string, port int) (bool, string) {
	imageID, err := m.run(ctx, "inspect", "--format", "{{.Image}}", m.coreContainer)
	if err != nil {
		return false, ""
	}
	publish := fmt.Sprintf("%s:%d:1", address, port)
	output, runErr := m.run(ctx, "run", "--rm", "--entrypoint", "true",
		"-p", publish+"/tcp", "-p", publish+"/udp", strings.TrimSpace(string(imageID)))
	if runErr == nil {
		return false, ""
	}
	// See runComposeUp: match against output and err.Error() together, not
	// just err.Error() alone, since an injected CommandRunner isn't
	// obligated to fold output into the error text the way the production
	// runner does.
	combined := runErr.Error() + "\n" + string(output)
	if !isPortBindConflict(strings.ToLower(combined)) {
		return false, ""
	}
	return true, strings.TrimSpace(combined)
}

var publishedPortPattern = regexp.MustCompile(`([0-9.]+|\[::\]):([0-9]+)->[0-9]+/(?:tcp|udp)`)

func occupiedDockerPort(output, requestedAddress string, requestedPort int) string {
	for _, line := range strings.Split(output, "\n") {
		parts := strings.SplitN(strings.TrimSpace(line), "|", 2)
		if len(parts) != 2 || strings.HasPrefix(parts[0], "rootguard-") {
			continue
		}
		for _, match := range publishedPortPattern.FindAllStringSubmatch(parts[1], -1) {
			port, _ := strconv.Atoi(match[2])
			if port != requestedPort {
				continue
			}
			address := match[1]
			if requestedAddress == "0.0.0.0" || address == "0.0.0.0" || address == "[::]" || address == requestedAddress {
				return parts[0]
			}
		}
	}
	return ""
}

func (m *Manager) load() {
	data, err := os.ReadFile(filepath.Join(m.dataDir, "status.json"))
	if err != nil {
		return
	}
	var status Status
	if json.Unmarshal(data, &status) == nil && status.State != "" {
		m.status = status
	}
}

func (m *Manager) persist() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.persistLocked()
}

func (m *Manager) persistLocked() error {
	if err := os.MkdirAll(m.dataDir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m.status, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(m.dataDir, "status.json")
	temp := path + ".tmp"
	if err := os.WriteFile(temp, data, 0600); err != nil {
		return err
	}
	return os.Rename(temp, path)
}

func cloneStatus(status Status) Status {
	clone := status
	if status.Config != nil {
		config := *status.Config
		clone.Config = &config
	}
	if status.Diagnostic != nil {
		diagnostic := *status.Diagnostic
		clone.Diagnostic = &diagnostic
	}
	clone.Steps = make([]Step, len(status.Steps))
	copy(clone.Steps, status.Steps)
	return clone
}

func runDocker(ctx context.Context, arguments ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, "docker", arguments...)
	output, err := command.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("docker %s: %w: %s", strings.Join(arguments, " "), err, strings.TrimSpace(string(output)))
	}
	return output, nil
}
