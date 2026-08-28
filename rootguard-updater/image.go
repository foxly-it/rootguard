package main

import (
	"context"
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
	repo, _, ok := strings.Cut(image, ":")
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
	repo, _, ok := strings.Cut(image, ":")
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
