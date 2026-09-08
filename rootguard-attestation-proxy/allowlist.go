package main

// allowedHosts is the complete, hardcoded set of hosts this proxy will
// ever tunnel to, port 443 only - not a general-purpose forward proxy.
// The first three were empirically confirmed live (round of 2026-09-01,
// cutting 1.0.0-rc.2): traced every host a real, successful `cosign
// verify-attestation` call actually contacts, via HTTPS_PROXY pointed at
// a local CONNECT-logging forwarding proxy, once with a completely cold
// TUF cache to also catch first-run-only bootstrap traffic. No live call
// to fulcio.sigstore.dev or rekor.sigstore.dev happens, since cosign
// verifies both the certificate chain and the transparency log inclusion
// proof offline, from data embedded in the attestation bundle itself
// plus the TUF-provided trust root.
//
//   - ghcr.io: the registry API - resolves the image/attestation
//     reference cosign is asked to verify.
//   - pkg-containers.githubusercontent.com: GHCR's actual blob storage
//     CDN, where the registry API redirects for the real attestation
//     manifest/layer content.
//   - tuf-repo-cdn.sigstore.dev: bootstraps/refreshes cosign's local
//     Sigstore trust root (Fulcio CA chain, Rekor public key, CT log
//     key) - needed even offline-verification-wise, since the trust
//     anchors themselves come from here, not the attestation bundle.
//   - api.github.com: added for Core's GitHub Releases self-update
//     discovery (internal/updater/github_release.go) - found in review
//     that this check ran on the same internal-only `control` network
//     as cosign but was never routed through this proxy, so it silently
//     never worked in the real release topology. Reuses this proxy
//     instead of standing up a second, separate metadata proxy for one
//     more host.
//
// This list is not an authentication boundary (see the trust-model note
// in proxy.go) - it exists so `control`'s own network isolation stays
// provably total except for this one, narrow, auditable, purpose-built
// path, rather than reopening it wholesale.
var allowedHosts = map[string]bool{
	"ghcr.io":                              true,
	"pkg-containers.githubusercontent.com": true,
	"tuf-repo-cdn.sigstore.dev":            true,
	"api.github.com":                       true,
}

const allowedPort = "443"

func isAllowed(host, port string) bool {
	return port == allowedPort && allowedHosts[host]
}
