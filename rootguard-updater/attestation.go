package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// attestationImagePrefix covers exactly this updater's two managed
// services (see newManager's specs in main) - both published by the same
// release-alpha.yml matrix build as every other RootGuard component.
// Unlike Core's own updater (which also manages AdGuard, a third-party
// image with no RootGuard signing policy and an explicit not_applicable
// exemption - see rootguard-core/internal/stack.RequireAttestation), this
// module never has a service that's legitimately exempt: everything it
// activates must come from a digest-pinned, cosign-verified image.
var attestationImagePrefix = map[string]string{
	"core":   "ghcr.io/foxly-it/rootguard-core",
	"webapp": "ghcr.io/foxly-it/rootguard-webapp",
}

const (
	attestationRepository = "foxly-it/rootguard"
	attestationIdentity   = `^https://github\.com/foxly-it/rootguard/\.github/workflows/release-alpha\.yml@refs/(tags/v[^/]+|heads/main)$`
)

type attestationVerifierFunc func(ctx context.Context, service, image string) error

// verifyAttestation gates activation, not just display - the same gap
// found and fixed in Core's own updater (see manager.go's
// AttestationVerifier there for the shared rationale): a pulled,
// digest-resolved image was activated the moment its post-swap health
// check passed, with no attestation check anywhere in between,
// contradicting docs/threat-model.md's explicit claim that releases are
// "checked via Cosign against the signed SLSA provenance before
// activation". This module can't import rootguard-core's internal/stack
// package (a different Go module entirely), so it carries its own
// minimal, standalone copy of the same cosign invocation instead of
// depending on one.
//
// The cosign subprocess's own environment is the one place this module
// ever routes traffic through rootguard-attestation-proxy - found live,
// cutting 1.0.0-rc.2: the Updater runs only on the `control` Docker
// network, deliberately `internal: true` (no route to the internet at
// all), so cosign's own outbound calls to GHCR/Sigstore can never
// succeed unmodified. See rootguard-core/internal/stack/attestation.go's
// identical fix and comment - same reasoning, same env var, scoped the
// same way (only this exec.Cmd's own Env, never the container's ambient
// environment).
func verifyAttestation(ctx context.Context, service, image string) error {
	prefix, ok := attestationImagePrefix[service]
	if !ok || !strings.HasPrefix(image, prefix) || !strings.Contains(image, "@sha256:") {
		return fmt.Errorf("%s (%s) is not eligible for attestation verification", service, image)
	}
	checkCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(checkCtx, "cosign", "verify-attestation",
		"--type", "https://slsa.dev/provenance/v1",
		"--certificate-identity-regexp", attestationIdentity,
		"--certificate-oidc-issuer", "https://token.actions.githubusercontent.com",
		"--certificate-github-workflow-repository", attestationRepository,
		image,
	)
	if proxyURL := os.Getenv("ROOTGUARD_ATTESTATION_PROXY_URL"); proxyURL != "" {
		cmd.Env = append(os.Environ(), "HTTPS_PROXY="+proxyURL, "HTTP_PROXY="+proxyURL)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s (%s): %s: %w", service, image, strings.TrimSpace(string(output)), err)
	}
	return nil
}
