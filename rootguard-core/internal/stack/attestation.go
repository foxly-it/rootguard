package stack

import (
	"context"
	"errors"
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
	"webapp":    releasePolicy("rootguard-webapp"),
	"updater":   releasePolicy("rootguard-updater"),
	"unbound":   releasePolicy("rootguard-unbound"),
	"blockpage": releasePolicy("rootguard-blockpage"),
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

func verifyReleaseAttestation(ctx context.Context, service, image string) (string, string) {
	return verifyReleaseAttestationWith(ctx, service, image, runAttestationCommand, time.Now)
}

func verifyReleaseAttestationWith(ctx context.Context, service, image string, run attestationRunner, now func() time.Time) (string, string) {
	policy, supported := attestationPolicies[service]
	if !supported || !strings.HasPrefix(image, policy.imagePrefix) || !strings.Contains(image, "@sha256:") {
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

func runAttestationCommand(ctx context.Context, name string, arguments ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, arguments...).CombinedOutput()
}
