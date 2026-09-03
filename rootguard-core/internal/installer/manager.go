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

	"github.com/foxly-it/rootguard-core/internal/atomicfile"
	"github.com/foxly-it/rootguard-core/internal/stack"
)

const (
	StateNotInstalled = "not_installed"
	StateDeploying    = "deploying"
	StateInstalled    = "installed"
	StateFailed       = "failed"

	// blockpagePort is the host port the blockpage container is published
	// on when config.BlockpageEnabled is set (see composeDNSFile's
	// "%s:80:8080/tcp" port mapping) - not configurable, so a plain
	// constant rather than a Config field.
	blockpagePort = 80
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
	// Level distinguishes an advisory check (OK is still true - it never
	// gates Preflight.Ready) from the pass/fail checks above: empty for
	// all of those, "warning" for a check like dockerCPPatchWarning below
	// that has a real Action worth surfacing but isn't confident enough to
	// block installation on. omitempty keeps every existing check's JSON
	// unchanged.
	Level string `json:"level,omitempty"`
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
	// PersistError/PersistErrorAt surface what onPersistError previously
	// only logged (see PersistErrorHandler's doc comment) - found in a
	// follow-up review: logging made a failed write *diagnosable* from the
	// container's own logs, but Status() itself still reported "installed"
	// (or whatever the in-memory state was) with no indication that the
	// on-disk record backing it might be stale - e.g. after a restart
	// following a full-disk write failure, the reported state could
	// silently regress with no visible cause. Set inside persistLocked on
	// failure, cleared the moment a later persistLocked call succeeds - so
	// it self-heals once the underlying disk/permissions problem does,
	// without any caller needing to do anything.
	PersistError   string    `json:"persist_error,omitempty"`
	PersistErrorAt time.Time `json:"persist_error_at,omitempty"`
}

type CommandRunner func(context.Context, ...string) ([]byte, error)
type BootstrapFunc func(context.Context, string) error
type RestoreFunc func(context.Context) error

// AttestationVerifierFunc gates activation, not just display - see
// AttestationVerifier's doc comment on Options.
type AttestationVerifierFunc func(ctx context.Context, service, image string) error

// PersistErrorHandler is called whenever persistLocked fails to write
// state to disk - found in review: nearly every one of persistLocked's
// many call sites discards its return value entirely (`_ =
// m.persistLocked()`), which on a full disk or a permissions problem
// meant an installation, restore, or migration step could report success
// in Status while its outcome was never actually written down. Rather
// than thread error handling through every one of those call sites
// individually, persistLocked calls this hook itself on failure -
// defaults to a no-op, so callers that want visibility (main.go logs it)
// can opt in without every internal package call needing to know about
// logging.
type PersistErrorHandler func(error)

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
	// OnPersistError is called whenever a state write fails - see
	// PersistErrorHandler's doc comment. Defaults to a no-op.
	OnPersistError PersistErrorHandler
	// AttestationVerifier gates activation of the first-ever DNS stack
	// deploy, the same way updater.Manager already gates every later
	// update - found in review: RequireAttestation existed and was wired
	// into both updater packages, but never into this one, so a fresh
	// install's very first Unbound/Blockpage activation (often the only
	// deployment event most installations ever have) skipped the check
	// docs/threat-model.md claims happens "before activation" entirely.
	// Defaults to stack.RequireAttestation; injectable purely so tests can
	// simulate a failed/missing attestation without a real cosign binary.
	AttestationVerifier AttestationVerifierFunc
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
	onPersistError         PersistErrorHandler
	attestationVerifier    AttestationVerifierFunc
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
	if options.OnPersistError == nil {
		options.OnPersistError = func(error) {}
	}
	if options.AttestationVerifier == nil {
		options.AttestationVerifier = stack.RequireAttestation
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
		onPersistError:         options.OnPersistError,
		attestationVerifier:    options.AttestationVerifier,
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
	if serverVersion, err := m.run(ctx, "version", "--format", "{{.Server.Version}}"); err != nil {
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
		if warning, ok := dockerCPPatchWarning(strings.TrimSpace(string(serverVersion))); ok {
			checks = append(checks, warning)
		}
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

	// Blockpage port 80 is a second, separate publication (composeDNSFile
	// below), independent of the DNS port and only made when
	// config.BlockpageEnabled is set - checked here too, not just at
	// deploy time, so a host that already has something bound to :80 (a
	// common case - many hosts run a web server) is caught by the
	// preflight instead of surfacing only after DNS deployment already
	// succeeded.
	if dockerOK && validNetworkConfig(config) && config.BlockpageEnabled {
		output, err := m.run(ctx, "ps", "--format", "{{.Names}}|{{.Ports}}")
		if err != nil {
			checks = append(checks, Check{
				ID: "blockpage_port_available", Code: "port_check_failed", OK: false,
				Message: "Docker port assignments could not be inspected.",
				Action:  "Verify Docker access and run the preflight again.",
			})
		} else if owner := occupiedDockerPort(string(output), config.DNSBindAddress, blockpagePort); owner != "" {
			checks = append(checks, Check{
				ID: "blockpage_port_available", Code: "blockpage_port_occupied", OK: false,
				Message: fmt.Sprintf("Blockpage port %s:%d is already published.", config.DNSBindAddress, blockpagePort),
				Detail:  owner,
				Action:  "Stop or reconfigure the conflicting service, or disable the blockpage, then run the preflight again.",
			})
		} else if busy, detail := m.probeHostPortBusy(ctx, config.DNSBindAddress, blockpagePort); busy {
			checks = append(checks, Check{
				ID: "blockpage_port_available", Code: "blockpage_port_occupied", OK: false,
				Message: fmt.Sprintf("Blockpage port %s:%d is already in use on this host.", config.DNSBindAddress, blockpagePort),
				Detail:  detail,
				Action:  "Stop or reconfigure the conflicting service - a web server listening on port 80 is a common cause - or disable the blockpage, then run the preflight again.",
			})
		} else {
			checks = append(checks, Check{
				ID: "blockpage_port_available", Code: "blockpage_port_available", OK: true,
				Message: "No conflicting port publication was found for the blockpage.",
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

// dockerCPFixedVersion is Docker Engine 29.5.1, the first upstream release
// with all three `docker cp` CVEs fixed: CVE-2026-41567 (arbitrary
// host-binary execution via PATH resolution during archive decompression),
// CVE-2026-41568 (a second, separate TOCTOU race letting a container
// create empty files/directories at an arbitrary host path - found in
// review: missing from this file's own original two-CVE list even though
// Docker's own 29.5.1 release notes cover all three together), and
// CVE-2026-42306 (a TOCTOU race that can redirect a bind-mount target to
// an arbitrary host path). Found in review: RootGuard itself calls
// `docker cp` in three places - backupexport, backuprestore, and
// updater's rollback path - so an unpatched host Docker Engine is a real
// exposure, not a theoretical one.
var dockerCPFixedVersion = [3]int{29, 5, 1}

// cleanDockerVersion matches only a plain upstream MAJOR.MINOR.PATCH
// version string (e.g. "29.4.0") - not one carrying a distro-packaging
// suffix (Debian/Ubuntu's docker.io package reports things like
// "24.0.7-1ubuntu1", e.g.). That distinction is deliberate: a distro can
// (and, per review, sometimes does) backport a security fix onto a
// package while keeping the same upstream-looking version number in its
// own suffixed form, which makes a suffixed version string just as
// uninformative about patch status as no version string at all. Only the
// unsuffixed, unambiguous case is one this function can respond to with
// any confidence.
var cleanDockerVersion = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)$`)

// dockerVersionLike loosely matches anything that at least attempts to
// look like a version string ("digits.digits...") without requiring
// cleanDockerVersion's strict unsuffixed form. Used only to decide
// whether an unparseable version is worth a distinct "patch level
// unknown" advisory (a real distro-suffixed version, e.g. - see
// cleanDockerVersion's own comment) versus staying fully silent for a
// value that plainly isn't a version string at all. Found in review:
// this repo's own test suite's fake CommandRunner stubs return a
// generic "ok" placeholder for any command they don't specifically
// care about, `docker version` included - that string not matching
// this looser pattern either is exactly why none of those tests needed
// updating for the new advisory below.
var dockerVersionLike = regexp.MustCompile(`^\d+\.\d+`)

// dockerCPPatchWarning reports an advisory (Check.OK stays true - see
// Check.Level's own doc comment) about the Docker Engine version
// Preflight just observed, relative to dockerCPFixedVersion. ok is false
// only when the version reads as confidently already patched - silence
// is the correct outcome there. Otherwise this returns one of two
// distinct advisories: a real warning when the version unambiguously
// predates the fix, or a lower-confidence "patch level unknown" notice
// when the version merely looks version-shaped but couldn't be read with
// that confidence at all (found in review: previously indistinguishable
// from "confirmed patched" - both produced total silence, even though
// "we genuinely can't tell" is worth surfacing differently from "we
// checked, it's fine"). Neither ever fails Ready - this can only ever
// warn, precisely because a false positive here has no real cost while a
// false negative only means the same information the CVEs are already
// public with.
func dockerCPPatchWarning(version string) (Check, bool) {
	m := cleanDockerVersion.FindStringSubmatch(version)
	if m == nil {
		if !dockerVersionLike.MatchString(version) {
			return Check{}, false
		}
		return Check{
			ID: "docker_engine_patch_level", Code: "docker_engine_cp_cve_unknown", OK: true, Level: "warning",
			Message: "Docker Engine's reported version couldn't be read with confidence, so whether it has the docker cp fixes (CVE-2026-41567, CVE-2026-41568, CVE-2026-42306, all fixed in 29.5.1) is unknown",
			Detail:  version,
			Action:  "Confirm your Docker Engine is 29.5.1 or later, or that your distribution has backported these fixes.",
		}, true
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	patch, _ := strconv.Atoi(m[3])
	fixedMajor, fixedMinor, fixedPatch := dockerCPFixedVersion[0], dockerCPFixedVersion[1], dockerCPFixedVersion[2]
	patched := major > fixedMajor ||
		(major == fixedMajor && minor > fixedMinor) ||
		(major == fixedMajor && minor == fixedMinor && patch >= fixedPatch)
	if patched {
		return Check{}, false
	}
	return Check{
		ID: "docker_engine_patch_level", Code: "docker_engine_cp_cve", OK: true, Level: "warning",
		Message: "Docker Engine predates 29.5.1, which fixed three docker cp vulnerabilities (CVE-2026-41567, CVE-2026-41568, CVE-2026-42306) that RootGuard's backup, restore, and update-rollback paths rely on",
		Detail:  version,
		Action:  "Upgrade Docker Engine to 29.5.1 or later, or confirm your distribution has already backported these fixes independently of its reported version number.",
	}, true
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
	composePath, err = m.writeCompose(config, m.unboundImage, m.blockpageImage)
	if err != nil {
		return fail("prepare", err)
	}
	_ = m.setStep("prepare", "done", "Managed stack definition is ready")
	_ = m.setStep("pull", "running", "Downloading configured service images")
	if _, err = m.run(ctx, "compose", "--project-name", "rootguard-dns", "-f", composePath, "pull"); err != nil {
		return fail("pull", err)
	}
	_ = m.setStep("pull", "done", "Service images are available")
	var unboundImage, blockpageImage string
	composePath, unboundImage, blockpageImage, err = m.resolveAndPinDigests(ctx, config)
	if err != nil {
		return fail("create", err)
	}
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
	if err := m.verifyStackAttestation(ctx, config, unboundImage, blockpageImage); err != nil {
		return fail("start", err)
	}
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
	composePath, err := m.writeCompose(config, m.unboundImage, m.blockpageImage)
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

	composePath, unboundImage, blockpageImage, err := m.resolveAndPinDigests(ctx, config)
	if err != nil {
		m.fail("start", err)
		return
	}
	if err := m.verifyStackAttestation(ctx, config, unboundImage, blockpageImage); err != nil {
		m.fail("start", err)
		return
	}
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

func (m *Manager) writeCompose(config Config, unboundImage, blockpageImage string) (string, error) {
	if err := os.MkdirAll(m.dataDir, 0700); err != nil {
		return "", fmt.Errorf("create installation data directory: %w", err)
	}
	adGuardImage := m.adGuardImage
	if config.AdGuardChannel == "beta" {
		adGuardImage = m.adGuardBetaImage
	}
	content, err := renderCompose(config, unboundImage, adGuardImage, blockpageImage, m.dnsNetworkCIDR)
	if err != nil {
		return "", err
	}
	path := filepath.Join(m.dataDir, "compose.yaml")
	if err := atomicfile.WriteFile(path, []byte(content), 0600); err != nil {
		return "", fmt.Errorf("write stack definition: %w", err)
	}
	return path, nil
}

// resolveDigest turns a possibly-mutable "repo:tag" image reference into an
// immutable "repo@sha256:..." one, using the digest of the image that was
// just pulled locally. Found in review: verifyStackAttestation used to be
// called with the plain, mutable image references from Options
// (m.unboundImage/m.blockpageImage) - stack.RequireAttestation reports
// "not_applicable" (and therefore refuses to activate) for anything
// without an explicit "@sha256:" reference, so the very first deploy of a
// real, correctly-signed release always failed here, since a release only
// ever hands the installer a tag (or, for a release candidate, a
// commit-scoped tag - see release-alpha.yml). Mirrors
// rootguard-updater's own resolveTargetImage/digestQualify pattern for the
// exact same reason - this is a third by-hand copy of that ~15-line
// lookup (see internal/updater/github_release.go's digestQualify for the
// first two and why a shared module wasn't judged worth it), now needed
// here too since installer and updater are separate managers with their
// own CommandRunner wiring.
func (m *Manager) resolveDigest(ctx context.Context, image string) string {
	if strings.Contains(image, "@sha256:") {
		return image
	}
	repo, ok := imageRepo(image)
	if !ok {
		return image
	}
	// docker pull, not docker image inspect .RepoDigests - found in a
	// follow-up review: the old implementation inspected the local image
	// object's full .RepoDigests list and took the *first* entry matching
	// this repo. That list belongs to the local image as a whole, not to
	// this specific pull - if the same image ID has ever been associated
	// with more than one digest for this repo (a real, previously-hit
	// failure mode already documented on digestFromPullOutput below, the
	// exact pattern this now mirrors), the first match can silently be a
	// stale one instead of the digest actually pulled just now, letting a
	// correctly-attested but outdated image get pinned into the stack
	// definition. `docker pull` itself reports the digest of exactly what
	// it just pulled ("Digest: sha256:...", once pulling finishes) -
	// authoritative in a way a post-hoc inspect isn't. A second pull here
	// (compose already pulled everything as part of the "pull" step) is
	// deliberate, not redundant: it's the only way to get that
	// authoritative per-image answer, and Docker's own pull is a fast,
	// mostly no-op manifest check when the image is already present and
	// unchanged - the same cost internal/updater's own update() already
	// pays on every update via the identical pattern.
	output, err := m.run(ctx, "pull", image)
	if err == nil {
		if digestRef, ok := digestFromPullOutput(repo, output); ok {
			return digestRef
		}
	}
	// Fallback for an unexpected pull-output format (or the pull call
	// itself failing, e.g. a transient registry hiccup after compose's own
	// pull already succeeded) - the same best-effort inspect this function
	// used to always rely on, now demoted to a last resort rather than the
	// primary path.
	inspected, err := m.run(ctx, "image", "inspect", "--format", "{{range .RepoDigests}}{{.}}|{{end}}", image)
	if err != nil {
		return image
	}
	for _, digestRef := range strings.Split(strings.TrimSpace(string(inspected)), "|") {
		if strings.HasPrefix(digestRef, repo+"@") {
			return digestRef
		}
	}
	return image
}

// imageRepo returns the repository portion of a "repo:tag" reference,
// correctly handling a registry host:port prefix - e.g.
// "registry.example:5000/rootguard-unbound:tag" - which the previous
// strings.Cut(image, ":") (first colon) implementation mis-split into
// "registry.example" and "5000/rootguard-unbound:tag", silently breaking
// every digest lookup for any image reference naming a non-default
// registry with an explicit port. A colon only separates the tag if it
// appears after the last "/"; any colon before that (or without a
// following "/" at all) is part of the registry host:port, not a tag
// separator - the same rule Docker's own reference parser
// (distribution/reference) uses. ok is false only when there's no colon
// after the repository path at all (an already-bare, tagless reference),
// matching the previous strings.Cut behavior's own "not found" case.
func imageRepo(image string) (repo string, ok bool) {
	lastSlash := strings.LastIndex(image, "/")
	lastColon := strings.LastIndex(image, ":")
	if lastColon <= lastSlash {
		return "", false
	}
	return image[:lastColon], true
}

// digestFromPullOutput extracts the digest `docker pull` itself reports
// for the image it just pulled - see resolveDigest's own comment on why
// this is preferred over inspecting the local image object's
// .RepoDigests. repo must already be split from any tag (imageRepo
// above) - the registry:port mis-split this fixes here also affected the
// identical repo-splitting logic in rootguard-updater's and
// internal/updater's own copies of this function; fixed there too in
// this same change (see those files' own updated comments).
func digestFromPullOutput(repo string, output []byte) (string, bool) {
	for _, line := range strings.Split(string(output), "\n") {
		digest, ok := strings.CutPrefix(strings.TrimSpace(line), "Digest: ")
		if ok && strings.HasPrefix(digest, "sha256:") {
			return repo + "@" + digest, true
		}
	}
	return "", false
}

// resolveAndPinDigests resolves unbound's (and, when enabled, blockpage's)
// just-pulled image to its immutable digest and rewrites the stack
// definition to reference that digest instead of the original mutable
// tag - so every step from here on (attestation, create/up) is anchored to
// exactly the image that was inspected, not whatever the tag happens to
// point at if it moves in between. Must be called after the "pull" step
// succeeds (the digest lookup needs the image present locally) and before
// "create"/"start" so containers are actually built from the pinned
// reference, not the original tag-based compose file.
func (m *Manager) resolveAndPinDigests(ctx context.Context, config Config) (composePath, unboundImage, blockpageImage string, err error) {
	unboundImage = m.resolveDigest(ctx, m.unboundImage)
	blockpageImage = m.blockpageImage
	if config.BlockpageEnabled {
		blockpageImage = m.resolveDigest(ctx, m.blockpageImage)
	}
	composePath, err = m.writeCompose(config, unboundImage, blockpageImage)
	if err != nil {
		return "", "", "", fmt.Errorf("pin attested image digests into the stack definition: %w", err)
	}
	return composePath, unboundImage, blockpageImage, nil
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

// verifyStackAttestation gates the DNS stack's activation - see
// AttestationVerifier's doc comment on Options for why this exists.
// AdGuard has no RootGuard signing policy (a third-party image, see
// stack.attestationPolicies) and is never checked here, matching
// RequireAttestation's own no-op behavior for it; Blockpage is only
// checked when config.BlockpageEnabled, since it isn't part of the
// stack at all otherwise.
func (m *Manager) verifyStackAttestation(ctx context.Context, config Config, unboundImage, blockpageImage string) error {
	if err := m.attestationVerifier(ctx, "unbound", unboundImage); err != nil {
		return fmt.Errorf("attestation: %w", err)
	}
	if config.BlockpageEnabled {
		if err := m.attestationVerifier(ctx, "blockpage", blockpageImage); err != nil {
			return fmt.Errorf("attestation: %w", err)
		}
	}
	return nil
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
	case strings.HasPrefix(detail, "attestation:"):
		diagnostic.Code = "attestation_failed"
		diagnostic.Message = "The release attestation for a service image could not be verified."
		diagnostic.Action = "Verify the image reference and registry access, then retry once a signed release image is available."
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

// persistLocked writes state to disk, reporting any failure via
// onPersistError before returning it - see PersistErrorHandler's doc
// comment for why: most callers discard the returned error outright.
// Also records the outcome in m.status.PersistError/PersistErrorAt
// itself (see Status's own doc comment) - cleared before the write
// attempt so a success always reports (and persists) a clean state, set
// in the deferred failure branch so a failure is visible immediately,
// without needing a second call to notice it.
func (m *Manager) persistLocked() (returnErr error) {
	m.status.PersistError = ""
	m.status.PersistErrorAt = time.Time{}
	defer func() {
		if returnErr != nil {
			m.status.PersistError = returnErr.Error()
			m.status.PersistErrorAt = time.Now().UTC()
			m.onPersistError(returnErr)
		}
	}()
	if err := os.MkdirAll(m.dataDir, 0700); err != nil {
		return err
	}
	return atomicfile.WriteJSON(filepath.Join(m.dataDir, "status.json"), m.status)
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
