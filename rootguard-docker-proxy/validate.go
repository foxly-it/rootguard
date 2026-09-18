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
	Image      string     `json:"Image"`
	HostConfig hostConfig `json:"HostConfig"`
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
// stdout/stderr/stream from the query string, not the body. Rejecting
// stdin=1/true is the entire security boundary here: without it, nothing
// else about this call can grant a new capability or reach a new
// container, since {id} must already reference something that exists.
func validateAttach(r *http.Request, _ []byte) error {
	switch r.URL.Query().Get("stdin") {
	case "1", "true":
		return fmt.Errorf("attach with stdin is not allowed")
	}
	return nil
}

// knownExecTargets/knownExecCommands: Core's own code only ever execs
// into rootguard-blockpage, running one of exactly two commands (see
// rootguard-core/internal/installer/manager.go's renderBlockpageConf/
// reloadBlockpage call sites).
var knownExecTargets = map[string]bool{
	"rootguard-blockpage": true,
}

var knownExecCommands = [][]string{
	{"sh", "/docker-entrypoint.d/19-render-blockpage-conf.sh"},
	{"nginx", "-s", "reload"},
}

type execCreateBody struct {
	Cmd []string `json:"Cmd"`
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

func validateExecCreate(r *http.Request, body []byte) error {
	target := execTargetFromPath(r.URL.Path)
	if !knownExecTargets[target] {
		return fmt.Errorf("exec target %q is not on the allowlist", target)
	}
	var e execCreateBody
	if err := json.Unmarshal(body, &e); err != nil {
		return fmt.Errorf("invalid exec-create body: %w", err)
	}
	for _, known := range knownExecCommands {
		if cmdEqual(e.Cmd, known) {
			return nil
		}
	}
	return fmt.Errorf("exec command %v is not on the allowlist", e.Cmd)
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

type networkConnectBody struct {
	Container string `json:"Container"`
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
	return nil
}
