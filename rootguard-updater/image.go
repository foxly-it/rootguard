package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/mod/semver"
)

// targetImageFor returns targetImages[spec.Name] when set, falling back to
// spec's own static TargetImage pin otherwise.
func targetImageFor(spec serviceSpec, targetImages map[string]string) string {
	if image, ok := targetImages[spec.Name]; ok && image != "" {
		return image
	}
	return spec.TargetImage
}

// digestQualify turns a bare "repo:tag" override (as resolved by Core's
// live release discovery) into an immutable "repo@sha256:..." one, using
// the digest the image was just pulled at. Cosign attestation verification
// requires an explicit @sha256: reference and reports "not_applicable"
// without one; an already-qualified (static pin) target passes through
// unchanged, and a lookup failure falls back to the original reference
// rather than failing the check/update over a cosmetic gap.
//
// Kept in sync by hand with rootguard-core's identical copy
// (internal/updater/github_release.go, also digestQualify) - separate Go
// modules can't share an internal package, and standing up a third shared
// module was judged not worth it for ~30 lines of stable, rarely-changing
// logic. Check that file too if this one changes.
func digestQualify(ctx context.Context, run runner, image string) string {
	if strings.Contains(image, "@sha256:") {
		return image
	}
	repo, ok := imageRepo(image)
	if !ok {
		return image
	}
	output, err := run(ctx, "image", "inspect", "--format", "{{range .RepoDigests}}{{.}}|{{end}}", image)
	if err != nil {
		return image
	}
	for _, digestRef := range strings.Split(strings.TrimSpace(string(output)), "|") {
		if strings.HasPrefix(digestRef, repo+"@") {
			return digestRef
		}
	}
	return image
}

// digestFromPullOutput extracts the digest `docker pull` itself reports for
// the image it just pulled ("Digest: sha256:...", printed once pulling
// finishes) - authoritative for "what was just pulled" in a way
// digestQualify's RepoDigests lookup above isn't: RepoDigests belongs to
// the local image object as a whole, so if a repository ever has more than
// one digest recorded against a matching local image (observed for real in
// CI - a repository whose tag had recently moved to a new release still
// carried the previous release's digest in its RepoDigests list), the
// first-match loop there can silently return the *stale* one, making an
// available update look like "already current." Preferred over
// digestQualify when it can parse pull's own output; digestQualify stays
// as the fallback for an already-qualified static pin (whose "pull" prints
// the same digest back anyway) or an unexpected output format.
//
// Also kept in sync by hand with rootguard-core's identical copy - see
// digestQualify's own doc comment above for why.
func digestFromPullOutput(image string, output []byte) (string, bool) {
	repo, ok := imageRepo(image)
	if !ok {
		return "", false
	}
	for _, line := range strings.Split(string(output), "\n") {
		digest, ok := strings.CutPrefix(strings.TrimSpace(line), "Digest: ")
		if ok && strings.HasPrefix(digest, "sha256:") {
			return repo + "@" + digest, true
		}
	}
	return "", false
}

// imageRepo returns the repository portion of a "repo:tag" reference,
// correctly handling a registry host:port prefix - e.g.
// "registry.example:5000/rootguard-unbound:tag" - which the previous
// strings.Cut(image, ":") (first colon) used by both functions above
// mis-split into "registry.example" and "5000/rootguard-unbound:tag",
// silently breaking the digest lookup for that entire class of
// reference. Found in a follow-up review of rootguard-core/internal/
// installer's identical, newer copy of this same lookup - the bug
// predates that copy and was already live here. A colon only separates
// the tag if it appears after the last "/"; any colon before that (or
// without a following "/" at all) is part of the registry host:port, not
// a tag separator - the same rule Docker's own reference parser
// (distribution/reference) uses. ok is false only when there's no colon
// after the repository path at all (an already-bare, tagless reference),
// matching the previous strings.Cut behavior's own "not found" case.
//
// Kept in sync by hand with rootguard-core/internal/updater's identical
// copy - see digestQualify's own doc comment above for why.
func imageRepo(image string) (repo string, ok bool) {
	lastSlash := strings.LastIndex(image, "/")
	lastColon := strings.LastIndex(image, ":")
	if lastColon <= lastSlash {
		return "", false
	}
	return image[:lastColon], true
}

// repositoryOf returns image's repository (registry host/path, neither tag
// nor digest) - unlike imageRepo above (deliberately left untouched: kept
// in sync by hand with rootguard-core's copy for the pull/digest-
// qualification use case, which only ever sees a bare "repo:tag"), this
// also strips an "@sha256:..." digest suffix. Used only to validate a
// target_images override (see validateTargetOverrides) against untrusted
// input, which - unlike every value imageRepo itself is ever called with
// today - could arrive in either shape.
func repositoryOf(image string) string {
	if at := strings.IndexByte(image, '@'); at != -1 {
		image = image[:at]
	}
	if repo, ok := imageRepo(image); ok {
		return repo
	}
	return image
}

// errTargetOverrideNotAllowlisted is returned by validateTargetOverrides -
// mapped to 400 Bad Request in the HTTP handlers, distinct from every
// other manager error (which the handlers already map to 409/500).
var errTargetOverrideNotAllowlisted = errors.New("target image override is not in the repository this service is pinned to")

// validateTargetOverrides is the registry/repo allowlist check found
// missing in review: target_images (an override supplied in the POST body
// of /api/control-plane/check|update, see decodeTargetOverrides) used to
// reach `docker pull` completely unchecked - only the later
// attestationVerifier call gated *activation*, not the pull itself. Since
// `docker pull` runs against the host's dockerd over the Docker socket
// (not inside this container's own network namespace), a holder of
// ROOTGUARD_UPDATER_TOKEN could force the host dockerd to make arbitrary
// outbound connections to any registry - undermining the internet
// isolation the attestation-proxy + `internal: true` `control` network
// are specifically meant to guarantee, even though exploiting it already
// requires the same trust tier as Core itself. Every override must stay
// within the same repository as its service's own static TargetImage pin
// (the legitimate use - see StartCheck's own doc comment - only ever
// substitutes a live-resolved *version* of the identical image, never a
// different one), checked once here rather than at every pull call site.
func validateTargetOverrides(specs []serviceSpec, targetImages map[string]string) error {
	for name, image := range targetImages {
		for _, spec := range specs {
			if spec.Name != name {
				continue
			}
			if allowed := repositoryOf(spec.TargetImage); repositoryOf(image) != allowed {
				return fmt.Errorf("%w: %s override %q is not in repository %q", errTargetOverrideNotAllowlisted, name, image, allowed)
			}
		}
	}
	return nil
}

// isOlderReleaseVersion reports whether candidate is an older release than
// current, both given as bare version strings without a leading "v" (the
// form org.opencontainers.image.version carries, e.g. "0.1.0-beta.12").
// Uses full SemVer 2.0 precedence via golang.org/x/mod/semver rather than a
// RootGuard-specific "0.1.0-(alpha|beta).N" parser, so this keeps working
// correctly across any future version scheme change (0.2.0, 1.0.0,
// 1.0.0-rc.1, ...) instead of silently going "not comparable" - and
// therefore not blocking anything - the day the convention changes.
// Confirmed empirically for both today's scheme and hypothetical future
// ones: beta always outranks alpha at the same major.minor.patch
// (identifiers compare lexically), a higher build number within the same
// series wins (both-numeric identifiers compare numerically, not as
// strings, so beta.9 correctly loses to beta.12), and a release with no
// pre-release suffix outranks any pre-release of the same
// major.minor.patch (0.2.0 beats 0.1.0-beta.14; 1.0.0 beats 1.0.0-rc.1).
// comparable is false whenever either string isn't a valid semantic
// version - a local/dev build, a missing label - the caller should treat
// that as "can't tell, don't block."
func isOlderReleaseVersion(candidate, current string) (older, comparable bool) {
	candidateVersion := "v" + strings.TrimPrefix(candidate, "v")
	currentVersion := "v" + strings.TrimPrefix(current, "v")
	if !semver.IsValid(candidateVersion) || !semver.IsValid(currentVersion) {
		return false, false
	}
	return semver.Compare(candidateVersion, currentVersion) < 0, true
}
