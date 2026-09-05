package stack

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const attestationCacheTTL = 10 * time.Minute

type attestationPolicy struct {
	imagePrefix string
	repository  string
	identity    string
}

// releaseRepository and releaseWorkflowIdentity describe the single signer
// every RootGuard-built image shares: all 5 components are published by the
// same release-alpha.yml matrix build in the monorepo, so only the image
// itself differs between policies below.
const (
	releaseRepository       = "foxly-it/rootguard"
	releaseWorkflowIdentity = `^https://github\.com/foxly-it/rootguard/\.github/workflows/release-alpha\.yml@refs/(tags/v[^/]+|heads/main)$`
)

func releasePolicy(image string) attestationPolicy {
	return attestationPolicy{
		imagePrefix: "ghcr.io/foxly-it/" + image,
		repository:  releaseRepository,
		identity:    releaseWorkflowIdentity,
	}
}

var attestationPolicies = map[string]attestationPolicy{
	"core": releasePolicy("rootguard-core"),
	// Stale until now: this still pointed at the archived, read-only
	// per-component repo and its build.yml, both gone since the monorepo
	// migration - webapp is published by release-alpha.yml in
	// foxly-it/rootguard now, the same as core. A real, validly signed
	// release image would have failed this policy's repository/workflow
	// check, since cosign's actual certificate identity names the
	// monorepo, not the archived repo this used to require.
	"webapp":            releasePolicy("rootguard-webapp"),
	"updater":           releasePolicy("rootguard-updater"),
	"unbound":           releasePolicy("rootguard-unbound"),
	"blockpage":         releasePolicy("rootguard-blockpage"),
	"attestation-proxy": releasePolicy("rootguard-attestation-proxy"),
}

type attestationResult struct {
	status    string
	checkedAt string
	expires   time.Time
}

var attestationCache = struct {
	sync.Mutex
	items map[string]attestationResult
}{items: make(map[string]attestationResult)}

type attestationRunner func(context.Context, string, ...string) ([]byte, error)

// attestationRun is swapped out in tests so CheckStackAttestations's real
// wiring (which services it actually checks, not just what
// verifyReleaseAttestationWith itself can do in isolation) can be verified
// end-to-end without invoking the real cosign binary.
var attestationRun attestationRunner = runAttestationCommand

func verifyReleaseAttestation(ctx context.Context, service, image string) (string, string) {
	return verifyReleaseAttestationWith(ctx, service, image, attestationRun, time.Now)
}

// RequireAttestation gates activation, not just display: found in review
// that CheckStackAttestations above was only ever wired into the stack
// status API (what the dashboard shows), never into the updater's actual
// update() path - a pulled, digest-resolved image gets activated as soon
// as its post-swap health check passes, with no attestation check in
// between at all, contradicting docs/threat-model.md's explicit claim that
// releases are "checked via Cosign against the signed SLSA provenance
// before activation". Call this right before the point of no return
// (selectImage/compose up) instead. A service with no RootGuard signing
// policy (AdGuard - a third-party image, see attestationPolicies) has
// nothing to attest and is deliberately let through; every other service
// must come back "verified" and nothing else - "missing", "failed",
// "unavailable", and even an unexpected "not_applicable" (e.g. image not
// yet digest-qualified, which should never happen at this call site but
// fails closed if it somehow does) are all refused.
func RequireAttestation(ctx context.Context, service, image string) error {
	if _, supported := attestationPolicies[service]; !supported {
		return nil
	}
	// Checked directly here, not folded into the cached dashboard status
	// below (which stays exactly the existing five-value enum the
	// frontend already type-checks against - see
	// rootguard-webapp/frontend/src/api/client.ts) - a distinct,
	// specific error for exactly this one condition, surfaced only on
	// the activation-gating path an update actually fails on, not on
	// the dashboard's own read-only attestation display.
	if err := CheckAttestationProxyReachable(); err != nil {
		return fmt.Errorf("release attestation for %s (%s): %w", service, image, err)
	}
	status, _ := verifyReleaseAttestation(ctx, service, image)
	if status == "verified" {
		return nil
	}
	return fmt.Errorf("release attestation for %s (%s) is %s, refusing to activate", service, image, status)
}

// dialProxy is swapped out in tests so
// CheckAttestationProxyReachable's own logic can be verified without a
// real TCP dial.
var dialProxy = func(network, addr string) (net.Conn, error) {
	return net.DialTimeout(network, addr, 3*time.Second)
}

// CheckAttestationProxyReachable gives operators a specific, actionable
// diagnosis instead of cosign's own generic network-error text for the
// single most likely cause of an attestation failure on this network:
// found live, cutting 1.0.0-rc.2, that self-update can never deliver a
// compose-topology change (a new service, a new network, a new env
// var) to an existing installation - it only ever swaps container
// images in place (docs/release-process.md, "Self-update can never
// deliver a compose-topology change"). An operator who updated via the
// WebGUI alone from a release that predates rootguard-attestation-proxy
// ends up running the new attestation-gating code with the old
// topology: no proxy service, no `egress` network, no
// ROOTGUARD_ATTESTATION_PROXY_URL. Checked *before* invoking cosign at
// all - a bare "unavailable" from cosign's own network error doesn't
// tell an operator this is a known, structural gap with a known fix
// (reinstall or a manual compose.release.yaml refresh), not a transient
// hiccup worth simply retrying.
func CheckAttestationProxyReachable() error {
	proxyURL := os.Getenv("ROOTGUARD_ATTESTATION_PROXY_URL")
	if proxyURL == "" {
		return errors.New("no attestation proxy configured (ROOTGUARD_ATTESTATION_PROXY_URL is unset) - this installation's compose topology likely predates rootguard-attestation-proxy; a fresh install or a manual compose.release.yaml refresh is required, see docs/release-process.md")
	}
	target := strings.TrimPrefix(strings.TrimPrefix(proxyURL, "https://"), "http://")
	conn, err := dialProxy("tcp", target)
	if err != nil {
		return fmt.Errorf("attestation proxy configured (%s) but unreachable: %w - this installation's compose topology may be missing the rootguard-attestation-proxy service or its egress network; a fresh install or a manual compose.release.yaml refresh is required, see docs/release-process.md", proxyURL, err)
	}
	_ = conn.Close()
	return nil
}

func verifyReleaseAttestationWith(ctx context.Context, service, image string, run attestationRunner, now func() time.Time) (string, string) {
	policy, supported := attestationPolicies[service]
	// Anchored to "@" (every eligible image here is always digest-qualified,
	// enforced by the strings.Contains check right after) so a same-prefix
	// sibling image name (e.g. "rootguard-core-evil") can't pass the "core"
	// policy just because "rootguard-core" is a string-prefix of it - found
	// in review. Real exploitation still requires the image to separately
	// pass cosign verification against the exact release workflow identity
	// below, so this tightens a check rather than closing an active bypass.
	if !supported || !strings.HasPrefix(image, policy.imagePrefix+"@") || !strings.Contains(image, "@sha256:") {
		return "not_applicable", ""
	}

	attestationCache.Lock()
	if cached, ok := attestationCache.items[image]; ok && now().Before(cached.expires) {
		attestationCache.Unlock()
		return cached.status, cached.checkedAt
	}
	attestationCache.Unlock()

	checkCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	output, err := run(checkCtx, "cosign", "verify-attestation",
		"--type", "https://slsa.dev/provenance/v1",
		"--certificate-identity-regexp", policy.identity,
		"--certificate-oidc-issuer", "https://token.actions.githubusercontent.com",
		"--certificate-github-workflow-repository", policy.repository,
		image,
	)
	status := classifyAttestationResult(output, err)
	checkedAt := now().UTC().Format(time.RFC3339)
	attestationCache.Lock()
	attestationCache.items[image] = attestationResult{status: status, checkedAt: checkedAt, expires: now().Add(attestationCacheTTL)}
	attestationCache.Unlock()
	return status, checkedAt
}

func classifyAttestationResult(output []byte, err error) string {
	if err == nil {
		return "verified"
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return "unavailable"
	}
	lower := strings.ToLower(string(output) + " " + err.Error())
	if strings.Contains(lower, "no attestations") || strings.Contains(lower, "no signatures") ||
		strings.Contains(lower, "manifest unknown") || strings.Contains(lower, "not found") {
		return "missing"
	}
	if strings.Contains(lower, "connection") || strings.Contains(lower, "timeout") ||
		strings.Contains(lower, "temporary") || strings.Contains(lower, "tls handshake") {
		return "unavailable"
	}
	return "failed"
}

// runAttestationCommand's own subprocess environment is the one place
// this repository ever routes traffic through
// rootguard-attestation-proxy - found live, cutting 1.0.0-rc.2: Core
// runs only on the `control` Docker network, deliberately
// `internal: true` (no route to the internet at all), so cosign's own
// outbound calls to GHCR/Sigstore can never succeed unmodified. Setting
// HTTPS_PROXY/HTTP_PROXY only on this exec.Cmd's own Env (never on the
// container's ambient environment) scopes the proxy to exactly this one
// subprocess - Core's other outbound HTTP calls (e.g.
// internal/updater/github_release.go's GitHub Releases self-update
// check) are a separate, already-known, pre-existing gap on the same
// isolated network and must not be silently routed through the same
// narrow 3-host allowlist, which they don't fit.
// ROOTGUARD_ATTESTATION_PROXY_URL unset/empty (local dev's compose.yaml
// without the proxy service, unit tests, the integration/E2E fixtures)
// means no proxy env is set at all - unchanged, pre-proxy behavior.
func runAttestationCommand(ctx context.Context, name string, arguments ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, arguments...)
	if proxyURL := os.Getenv("ROOTGUARD_ATTESTATION_PROXY_URL"); proxyURL != "" {
		cmd.Env = append(os.Environ(), "HTTPS_PROXY="+proxyURL, "HTTP_PROXY="+proxyURL)
	}
	return cmd.CombinedOutput()
}
