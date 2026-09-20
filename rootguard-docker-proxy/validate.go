package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

// bodyValidator inspects one capability-granting request before it's
// allowed through to the real Docker socket. r is the incoming request
// (for its URL path/query - Docker puts the target resource ID and pull
// parameters there, not in the body); body is the already-buffered
// request body. A non-nil error rejects the request with 403 and the
// error text (logged server-side, not leaked to the caller in detail
// beyond "forbidden").
type bodyValidator func(r *http.Request, body []byte) error

// knownImageRepositories is every image repository this RootGuard release
// actually pulls or runs, compiled in rather than configurable - widening
// it requires a code change and a new release, same philosophy as
// rootguard-attestation-proxy's hardcoded host allowlist. Tags/digests are
// deliberately not part of this set (they change every release); only the
// repository identity is checked.
var knownImageRepositories = map[string]bool{
	"ghcr.io/foxly-it/rootguard-core":              true,
	"ghcr.io/foxly-it/rootguard-webapp":            true,
	"ghcr.io/foxly-it/rootguard-unbound":           true,
	"ghcr.io/foxly-it/rootguard-blockpage":         true,
	"ghcr.io/foxly-it/rootguard-updater":           true,
	"ghcr.io/foxly-it/rootguard-attestation-proxy": true,
	"adguard/adguardhome":                          true,
}

// bareDigestImageRef matches a resolved, content-addressable image
// reference with no repository at all - "sha256:" followed by exactly 64
// hex characters, what a container's own .Image/.Config.Image field and
// `docker image inspect --format {{.Id}}` return. rootguard-core's own
// port-probe (installer/manager.go's probeHostPortBusy) and volume-
// ownership-migration helpers (updater/manager.go) deliberately reuse an
// already-pulled image this way to avoid a redundant pull - found live
// wiring this proxy into the real stack, when validateContainerCreate
// rejected it because stripImageRef's tag-splitting logic read the
// "sha256" prefix as if it were a bare, slash-free repository name.
// Always allowed regardless of knownImageRepositories: Docker can only
// ever create a container from a digest it already has cached, and the
// only way image content enters this proxy's daemon at all is the
// repository-checked /images/create path below - no build/commit/load
// endpoint is on this allowlist - so a bare digest is transitively
// already vetted by the time anything could reference it this way.
var bareDigestImageRef = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// stripImageRef reduces "repo:tag", "repo@sha256:...", or a bare "repo"
// down to just the repository part, so a digest/tag change (expected
// every release) never requires touching knownImageRepositories.
func stripImageRef(ref string) string {
	// docker compose normalizes a bare Docker Hub reference like
	// "adguard/adguardhome" to its fully-qualified "docker.io/adguard/..."
	// form before issuing the real /images/create call - found live
	// wiring this proxy into the real stack. Every other known repository
	// is already registry-qualified (ghcr.io/...), so this is the only
	// one that needed it.
	ref = strings.TrimPrefix(ref, "docker.io/")
	if i := strings.Index(ref, "@"); i != -1 {
		ref = ref[:i]
	}
	// A ":" can also appear in a registry host:port (e.g.
	// "localhost:5000/repo") - only strip a trailing tag, i.e. a ":"
	// after the last "/".
	if i := strings.LastIndex(ref, "/"); i != -1 {
		if j := strings.Index(ref[i:], ":"); j != -1 {
			return ref[:i+j]
		}
		return ref
	}
	if j := strings.Index(ref, ":"); j != -1 {
		return ref[:j]
	}
	return ref
}

// validateImageCreate covers `POST /images/create` (docker pull / docker
// compose pull). Docker takes the image reference from the query string
// (`fromImage`, optionally `tag`), not the body.
func validateImageCreate(r *http.Request, _ []byte) error {
	fromImage := r.URL.Query().Get("fromImage")
	if fromImage == "" {
		return fmt.Errorf("missing fromImage parameter")
	}
	repo := stripImageRef(fromImage)
	if !knownImageRepositories[repo] {
		return fmt.Errorf("image repository %q is not on the allowlist", repo)
	}
	return nil
}

// hostConfig mirrors only the fields of Docker's HostConfig this proxy
// actually needs to police - everything else in a real HostConfig is
// ignored (and passed through unmodified, since the body itself is
// forwarded byte-for-byte once validated, never rewritten).
type hostConfig struct {
	Privileged  bool     `json:"Privileged"`
	NetworkMode string   `json:"NetworkMode"`
	PidMode     string   `json:"PidMode"`
	IpcMode     string   `json:"IpcMode"`
	UTSMode     string   `json:"UTSMode"`
	CapAdd      []string `json:"CapAdd"`
	Devices     []any    `json:"Devices"`
	Binds       []string `json:"Binds"`
	Mounts      []mount  `json:"Mounts"`
}

type mount struct {
	Type   string `json:"Type"`
	Source string `json:"Source"`
	Target string `json:"Target"`
}

type containerCreateBody struct {
	Image            string            `json:"Image"`
	Cmd              []string          `json:"Cmd"`
	Entrypoint       []string          `json:"Entrypoint"`
	User             string            `json:"User"`
	HostConfig       hostConfig        `json:"HostConfig"`
	NetworkingConfig *networkingConfig `json:"NetworkingConfig"`
}

// networkingConfig mirrors container-create's own network-attachment
// block - only the map's keys (which network(s) this call attaches to)
// are inspected; per-endpoint content (aliases, static IPs) isn't
// unmarshaled at all, same scope limit as validateNetworkConnect below.
type networkingConfig struct {
	EndpointsConfig map[string]json.RawMessage `json:"EndpointsConfig"`
}

// knownCapAdd is the union of every CapAdd RootGuard's own code or
// compose YAML ever actually requests: the chown-helper container
// (CHOWN only) and the blockpage service's compose-level cap_add
// (CHOWN, SETUID, SETGID, for nginx's root-to-worker privilege drop).
var knownCapAdd = map[string]bool{
	"CHOWN":  true,
	"SETUID": true,
	"SETGID": true,
}

// knownVolumeNames is every named Docker volume RootGuard's own compose
// files declare - a Bind/Mount source outside this set (most importantly,
// any host filesystem path) is exactly the "arbitrary bind mount = host
// compromise" primitive this proxy exists to close off.
var knownVolumeNames = map[string]bool{
	"rootguard-data":           true,
	"rootguard-sessions":       true,
	"unbound-config":           true,
	"adguard-auth":             true,
	"rootguard-unbound-config": true,
	"rootguard-unbound-state":  true,
	"rootguard-adguard-work":   true,
	"rootguard-adguard-config": true,
	"rootguard-adguard-auth":   true,
}

// validateContainerCreate covers `POST /containers/create` - the one call
// that can request a privileged container or an arbitrary host bind
// mount. Rejects anything that isn't one of RootGuard's own known images
// with a tightly bounded HostConfig.
func validateContainerCreate(_ *http.Request, body []byte) error {
	var c containerCreateBody
	if err := json.Unmarshal(body, &c); err != nil {
		return fmt.Errorf("invalid container-create body: %w", err)
	}

	if !bareDigestImageRef.MatchString(c.Image) {
		repo := stripImageRef(c.Image)
		if !knownImageRepositories[repo] {
			return fmt.Errorf("image repository %q is not on the allowlist", repo)
		}
	}

	if err := validateContainerProcess(c); err != nil {
		return err
	}
	if err := validateContainerNetworking(c.NetworkingConfig); err != nil {
		return err
	}

	hc := c.HostConfig
	if hc.Privileged {
		return fmt.Errorf("privileged containers are not allowed")
	}
	for _, mode := range []string{hc.NetworkMode, hc.PidMode, hc.IpcMode, hc.UTSMode} {
		if mode == "host" {
			return fmt.Errorf("host namespace sharing is not allowed")
		}
	}
	if len(hc.Devices) > 0 {
		return fmt.Errorf("device mappings are not allowed")
	}
	for _, capName := range hc.CapAdd {
		// Found live wiring this proxy into the real stack: this Docker
		// version sends the kernel-style "CAP_CHOWN" form in the actual
		// API request body, not the short "CHOWN" form compose.release.yaml's
		// own cap_add: [CHOWN, ...] YAML and knownCapAdd both use -
		// normalize before comparing rather than doubling every entry.
		normalized := strings.TrimPrefix(strings.ToUpper(capName), "CAP_")
		if !knownCapAdd[normalized] {
			return fmt.Errorf("capability %q is not on the allowlist", capName)
		}
	}
	if err := validateBinds(hc.Binds); err != nil {
		return err
	}
	if err := validateMounts(hc.Mounts); err != nil {
		return err
	}
	return nil
}

// knownProcessOverrides: the only two container-create calls Core's own
// code issues with an explicit Cmd/Entrypoint/User - probeHostPortBusy's
// port-probe (installer/manager.go) and normalizeUnboundOwnership's
// chown-helper (backuprestore/manager.go). Every other container Core/the
// Updater ever create - the entire compose-managed stack - never
// overrides these fields at all and relies purely on the image's own
// defaults, which is why an all-empty Cmd/Entrypoint/User is accepted
// unconditionally below. Found in review: an earlier version of this
// proxy validated Image and HostConfig only, on the mistaken assumption
// that those were the only fields capable of granting something new -
// Cmd/Entrypoint/User are top-level fields on the same request and were
// forwarded completely unchecked, letting an already-allowed image run
// arbitrary code via a crafted Entrypoint/Cmd.
var knownProcessOverrides = []struct {
	entrypoint []string
	user       string
	cmds       [][]string
}{
	{
		entrypoint: []string{"true"},
		user:       "",
		cmds:       [][]string{nil},
	},
	{
		entrypoint: []string{"/usr/bin/chown"},
		user:       "0:0",
		cmds: [][]string{
			{"--recursive", "100:101", "/etc/unbound/unbound.d"},
			{"--recursive", "100:101", "/var/lib/unbound"},
		},
	},
}

func validateContainerProcess(c containerCreateBody) error {
	if len(c.Cmd) == 0 && len(c.Entrypoint) == 0 && c.User == "" {
		return nil
	}
	for _, known := range knownProcessOverrides {
		if !cmdEqual(c.Entrypoint, known.entrypoint) || c.User != known.user {
			continue
		}
		for _, cmd := range known.cmds {
			if cmdEqual(c.Cmd, cmd) {
				return nil
			}
		}
	}
	return fmt.Errorf("container create with Cmd=%v Entrypoint=%v User=%q is not on the allowlist", c.Cmd, c.Entrypoint, c.User)
}

// knownComposeNetworks: every network a container-create call from
// Core/the Updater can legitimately request. edge/control/egress are
// Compose-managed with no explicit `name:` in compose.release.yaml, so
// Compose auto-names them "<project>_<key>" - this stack's project name
// is pinned to "rootguard" (compose.release.yaml's own top-level
// `name:`). rootguard-dns is Core's own generated compose file for
// AdGuard/Unbound/blockpage, which does give it an explicit fixed name.
// "default" isn't a real named network at all - found live: the port-
// probe's plain `docker run` (installer/manager.go's probeHostPortBusy,
// no `--network` flag at all) has the Docker CLI itself populate
// NetworkingConfig.EndpointsConfig with this literal key, which the
// daemon resolves to its own default bridge network; every other known
// process override either sets `--network none` (no NetworkingConfig at
// all) or is compose-managed (a real network name, never this one). Per-
// endpoint content (aliases, static IPs) within an entry isn't policed
// here, the same scope limit already accepted for validateNetworkConnect
// below - this bounds *which* network a create call can join, not what
// identity it claims there.
var knownComposeNetworks = map[string]bool{
	"default":           true,
	"rootguard_control": true,
	"rootguard_edge":    true,
	"rootguard_egress":  true,
	"rootguard-dns":     true,
}

func validateContainerNetworking(nc *networkingConfig) error {
	if nc == nil {
		return nil
	}
	for name := range nc.EndpointsConfig {
		if !knownComposeNetworks[name] {
			return fmt.Errorf("network %q is not on the allowlist", name)
		}
	}
	return nil
}

// validateBinds checks legacy "source:target[:mode]" bind-mount strings.
// Only a known named volume may appear as the source - a bare host path
// (starting with "/") is exactly the primitive this proxy exists to
// close off, so it's always rejected regardless of what's on the other
// side of the colon.
func validateBinds(binds []string) error {
	for _, b := range binds {
		parts := strings.SplitN(b, ":", 3)
		source := parts[0]
		if !knownVolumeNames[source] {
			return fmt.Errorf("bind mount source %q is not an allow-listed named volume", source)
		}
	}
	return nil
}

func validateMounts(mounts []mount) error {
	for _, m := range mounts {
		if m.Type != "volume" {
			// "bind" mounts a host path directly; "tmpfs"/others aren't
			// used by any known RootGuard call site either.
			return fmt.Errorf("mount type %q is not allowed", m.Type)
		}
		if !knownVolumeNames[m.Source] {
			return fmt.Errorf("mount source %q is not an allow-listed named volume", m.Source)
		}
	}
	return nil
}

// validateAttach covers `POST /containers/{id}/attach` - see allowlist.go's
// comment on this rule for why it's needed at all. Docker takes stdin/
// stdout/stderr/stream from the query string, not the body. Rejecting a
// truthy stdin is the entire security boundary here: without it, nothing
// else about this call can grant a new capability or reach a new
// container, since {id} must already reference something that exists.
// Matches Docker's own httputils.BoolValue exactly (case-insensitive,
// trimmed; only "", "0", "no", "false", "none" are falsy) - found in
// review: an earlier version only rejected the literal strings "1"/"true",
// so "stdin=True" or "stdin=yes" reached the real daemon unrejected and
// opened the container's stdin anyway.
func validateAttach(r *http.Request, _ []byte) error {
	switch strings.ToLower(strings.TrimSpace(r.URL.Query().Get("stdin"))) {
	case "", "0", "no", "false", "none":
		return nil
	}
	return fmt.Errorf("attach with stdin is not allowed")
}

// AttachStdin/Privileged/User are additionally checked below even though
// no currently-allowed exec command needs any of them: Privileged grants
// every Linux capability to the exec'd process unconditionally
// (independent of the container's own HostConfig.Privileged), User can
// run the command as an arbitrary UID, and AttachStdin - mirrored from
// validateAttach's own reasoning - is what makes a later
// `POST /exec/{id}/start` (unconditionally allowed once exec-create
// succeeds) able to write into the running command at all.
type execCreateBody struct {
	Cmd         []string `json:"Cmd"`
	Privileged  bool     `json:"Privileged"`
	User        string   `json:"User"`
	AttachStdin bool     `json:"AttachStdin"`
}

// execTargetFromPath extracts the {id} segment from
// "/containers/{id}/exec" (already version-stripped by the caller).
func execTargetFromPath(path string) string {
	trimmed := strings.TrimPrefix(normalizePath(path), "/containers/")
	if i := strings.Index(trimmed, "/"); i != -1 {
		return trimmed[:i]
	}
	return ""
}

// Core's own code execs into exactly two containers - rootguard-blockpage
// (reloading its nginx config) and rootguard-unbound (config-syntax
// checks and read-only diagnostics; see rootguard-core/internal/unbound).
// Found live: an earlier version of this allowlist only ever covered
// rootguard-blockpage, on the mistaken assumption that it was the only
// exec target - it silently broke every Unbound guided-setting change,
// custom-config edit, and diagnostic once docker-proxy sat in the
// request path, none of which any CI fixture happened to exercise. Fixed
// by actually enumerating every real exec call site in rootguard-core
// (grep for `"exec"` across the module) and validating each rather than
// guessing again.
func validateExecCreate(r *http.Request, body []byte) error {
	target := execTargetFromPath(r.URL.Path)
	var e execCreateBody
	if err := json.Unmarshal(body, &e); err != nil {
		return fmt.Errorf("invalid exec-create body: %w", err)
	}
	if e.Privileged {
		return fmt.Errorf("privileged exec is not allowed")
	}
	if e.User != "" {
		return fmt.Errorf("exec with a User override is not allowed")
	}
	if e.AttachStdin {
		return fmt.Errorf("exec with AttachStdin is not allowed")
	}
	switch target {
	case "rootguard-blockpage":
		return validateBlockpageExecCommand(e.Cmd)
	case "rootguard-unbound":
		return validateUnboundExecCommand(e.Cmd)
	default:
		return fmt.Errorf("exec target %q is not on the allowlist", target)
	}
}

// rootguard-core/internal/installer/manager.go's renderBlockpageConf/
// reloadBlockpage call sites - exactly two fixed commands, no arguments
// ever vary.
var knownBlockpageExecCommands = [][]string{
	{"sh", "/docker-entrypoint.d/19-render-blockpage-conf.sh"},
	{"nginx", "-s", "reload"},
}

func validateBlockpageExecCommand(cmd []string) error {
	for _, known := range knownBlockpageExecCommands {
		if cmdEqual(cmd, known) {
			return nil
		}
	}
	return fmt.Errorf("exec command %v is not on the allowlist", cmd)
}

// Core already has full, unrestricted control over rootguard-unbound's
// actual behavior - it writes every config file Unbound reads (bind
// mounts, not exec) and can `docker restart` it outright - so none of
// the exec calls below grant it anything it doesn't already have. Each
// is still validated by shape rather than accepted as an opaque command,
// since exec remains the one call that runs arbitrary code inside a
// container rather than a fixed Docker API verb. unbound-checkconf/cat
// only ever target two fixed paths each; unbound-control and dig take
// runtime-variable arguments (a verbosity level, an operator-configured
// forward-check zone/address, AdGuard's own dynamically-assigned
// container IP) that a literal command list can't express, so those two
// are validated structurally instead - see rootguard-core/internal/unbound
// for every real call site this was built from (custom.go, diagnostic_
// logging.go, forwarding.go, lifecycle.go, network.go, settings.go).
var knownUnboundFixedExecCommands = [][]string{
	{"unbound-checkconf", "/etc/unbound/unbound.conf"},
	{"unbound-checkconf", "/etc/unbound/unbound.d/.rootguard-combined.candidate"},
	{"cat", "/etc/unbound/unbound.conf"},
	{"cat", "/etc/unbound/unbound.d/50-rootguard.conf"},
	{"unbound-control", "status"},
	{"dig", "@127.0.0.1", "-p", "5335", ".", "NS", "+time=1", "+tries=1"},
	{"dig", "-4", "+time=2", "+tries=1", "+short", "@198.41.0.4", ".", "NS"},
	{"dig", "-6", "+time=2", "+tries=1", "+short", "@2001:503:ba3e::2:30", ".", "NS"},
}

// unboundVerbosity: Unbound's own verbosity levels only go from 0 (no
// extra logging) to 5 (very noisy) - see settings.go's LogVerbosity field.
var unboundVerbosity = regexp.MustCompile(`^[0-5]$`)

func validateUnboundExecCommand(cmd []string) error {
	for _, known := range knownUnboundFixedExecCommands {
		if cmdEqual(cmd, known) {
			return nil
		}
	}
	if len(cmd) == 3 && cmd[0] == "unbound-control" && cmd[1] == "verbosity" && unboundVerbosity.MatchString(cmd[2]) {
		return nil
	}
	if len(cmd) > 0 && cmd[0] == "dig" {
		return validateUnboundDig(cmd[1:])
	}
	return fmt.Errorf("exec command %v is not on the allowlist", cmd)
}

// digFlag covers every dig flag Core's own call sites actually pass
// (lifecycle.go's digTimeout/tries constants, forwarding.go's fixed
// output-shaping flags) - +time=N and +tries=N are bounded to two digits
// since Core never waits longer than the low tens of seconds for a
// diagnostic query.
var digFlag = regexp.MustCompile(`^\+(short|dnssec|noall|comments|answer|authority|time=[0-9]{1,2}|tries=[0-9]{1,2})$`)

// digQTypes: the only record types any Core call site ever queries for.
var digQTypes = map[string]bool{"A": true, "NS": true, "SOA": true}

// digName: a conservative DNS label/name shape - covers "." (the root,
// used by the connectivity probes), the two fixed diagnostic domains
// (example.com/dnssec-failed.org, overridable in CI only, see
// SetDiagnosticDomains), and any zone an operator has configured as a
// forward target (forwarding.go's checkForwardTarget - already fully
// operator-controlled via the existing, sanctioned /api/unbound/settings
// forward-zone feature, so accepting any well-formed zone name here
// grants nothing beyond what that feature already allows).
var digName = regexp.MustCompile(`^\.$|^[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?(?:\.[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?)*\.?$`)

// validateUnboundDig covers every `dig` invocation Core makes against
// rootguard-unbound (root-server connectivity probes, the direct and
// AdGuard-path resolution/DNSSEC diagnostics, and per-forward-zone
// checks). The server and query-name tokens are necessarily variable
// (AdGuard's container IP is assigned dynamically; a forward zone/address
// is whatever the operator configured), so this validates the shape of
// the command - a known flag, `-p <numeric port>`, an `@server` token, a
// query name, and a known record type - rather than one fixed argument
// list per call site.
func validateUnboundDig(args []string) error {
	var server, name, qtype string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case digFlag.MatchString(a):
		case a == "-4" || a == "-6":
		case a == "-p":
			i++
			if i >= len(args) || !isNumeric(args[i]) {
				return fmt.Errorf("dig -p requires a numeric port")
			}
		case strings.HasPrefix(a, "@") && len(a) > 1:
			server = a
		case digQTypes[a]:
			qtype = a
		case name == "" && digName.MatchString(a):
			name = a
		default:
			return fmt.Errorf("dig argument %q is not on the allowlist", a)
		}
	}
	if server == "" {
		return fmt.Errorf("dig command has no @server target")
	}
	if name == "" {
		return fmt.Errorf("dig command has no query name")
	}
	if qtype == "" {
		return fmt.Errorf("dig command has no recognized query type")
	}
	return nil
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func cmdEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// knownNetworkTargets/knownNetworkConnectContainers: Core's own code only
// ever connects itself to rootguard-dns
// (rootguard-core/internal/installer/manager.go's joinDNSNetwork).
var knownNetworkTargets = map[string]bool{
	"rootguard-dns": true,
}

var knownNetworkConnectContainers = map[string]bool{
	"rootguard-core": true,
}

// EndpointConfig lets a connect call additionally claim a DNS alias or a
// fixed IP on the network it's joining. Core's only real use
// (connectCoreToDNSNetwork) sets exactly one thing - IPAMConfig.IPv4Address,
// a plain dotted-quad derived from the DNS network's own CIDR - so
// everything else here is rejected rather than forwarded unchecked; found
// in review, this field was previously ignored entirely.
type endpointIPAMConfig struct {
	IPv4Address  string   `json:"IPv4Address"`
	IPv6Address  string   `json:"IPv6Address"`
	LinkLocalIPs []string `json:"LinkLocalIPs"`
}

type endpointConfig struct {
	IPAMConfig *endpointIPAMConfig `json:"IPAMConfig"`
	Links      []string            `json:"Links"`
	Aliases    []string            `json:"Aliases"`
	MacAddress string              `json:"MacAddress"`
	DriverOpts map[string]string   `json:"DriverOpts"`
}

var ipv4DottedQuad = regexp.MustCompile(`^(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)(\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)){3}$`)

func validateEndpointConfig(ec *endpointConfig) error {
	if ec == nil {
		return nil
	}
	if len(ec.Links) > 0 || len(ec.Aliases) > 0 || ec.MacAddress != "" || len(ec.DriverOpts) > 0 {
		return fmt.Errorf("endpoint config may only set an IPv4 address")
	}
	if ec.IPAMConfig == nil {
		return nil
	}
	if ec.IPAMConfig.IPv6Address != "" || len(ec.IPAMConfig.LinkLocalIPs) > 0 {
		return fmt.Errorf("endpoint config may only set an IPv4 address")
	}
	if ec.IPAMConfig.IPv4Address != "" && !ipv4DottedQuad.MatchString(ec.IPAMConfig.IPv4Address) {
		return fmt.Errorf("endpoint IPv4 address %q is not well-formed", ec.IPAMConfig.IPv4Address)
	}
	return nil
}

type networkConnectBody struct {
	Container      string          `json:"Container"`
	EndpointConfig *endpointConfig `json:"EndpointConfig"`
}

func networkTargetFromPath(path string) string {
	trimmed := strings.TrimPrefix(normalizePath(path), "/networks/")
	if i := strings.Index(trimmed, "/"); i != -1 {
		return trimmed[:i]
	}
	return ""
}

func validateNetworkConnect(r *http.Request, body []byte) error {
	target := networkTargetFromPath(r.URL.Path)
	if !knownNetworkTargets[target] {
		return fmt.Errorf("network %q is not on the allowlist", target)
	}
	var n networkConnectBody
	if err := json.Unmarshal(body, &n); err != nil {
		return fmt.Errorf("invalid network-connect body: %w", err)
	}
	if !knownNetworkConnectContainers[n.Container] {
		return fmt.Errorf("container %q is not allowed to join %q", n.Container, target)
	}
	return validateEndpointConfig(n.EndpointConfig)
}
