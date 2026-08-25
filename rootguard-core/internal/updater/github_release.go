package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

// releaseTagPattern matches RootGuard's own release-tag convention, the same
// pattern release-alpha.yml's own "version" job validates new tags against.
// The two numbered capture groups (series, build number) let
// pickLatestReleaseImage below rank releases itself instead of trusting the
// API's own ordering.
var releaseTagPattern = regexp.MustCompile(`^v0\.1\.0-(alpha|beta)\.([0-9]+)$`)

type githubRelease struct {
	TagName string `json:"tag_name"`
}

// pickLatestReleaseImage scans a GitHub Releases API response body for the
// newest RootGuard release tag and returns the matching ghcr.io image
// reference for component. Ranks every matching release itself by
// (series, build number) rather than trusting the API response's own
// ordering - found via a real CI failure: querying the live API directly
// showed v0.1.0-beta.9 listed *ahead of* v0.1.0-beta.12, on a repository
// that had several releases cut in quick succession, so "the full,
// newest-first list" this function's doc comment used to promise isn't
// actually guaranteed. beta ranks above alpha on a tie (RootGuard's series
// only ever moves alpha -> beta, never back).
func pickLatestReleaseImage(body []byte, component string) (string, error) {
	var releases []githubRelease
	if err := json.Unmarshal(body, &releases); err != nil {
		return "", fmt.Errorf("parse GitHub releases response: %w", err)
	}
	var bestVersion string
	var bestSeriesRank, bestBuild int
	found := false
	for _, release := range releases {
		match := releaseTagPattern.FindStringSubmatch(release.TagName)
		if match == nil {
			continue
		}
		seriesRank := 0
		if match[1] == "beta" {
			seriesRank = 1
		}
		build, err := strconv.Atoi(match[2])
		if err != nil {
			continue
		}
		if found && (seriesRank < bestSeriesRank || (seriesRank == bestSeriesRank && build <= bestBuild)) {
			continue
		}
		bestSeriesRank, bestBuild, found = seriesRank, build, true
		bestVersion = strings.TrimPrefix(release.TagName, "v")
	}
	if !found {
		return "", fmt.Errorf("no matching RootGuard release found")
	}
	return fmt.Sprintf("ghcr.io/foxly-it/rootguard-%s:%s", component, bestVersion), nil
}

// ResolveLatestReleaseImage queries the public GitHub Releases API for
// foxly-it/rootguard and resolves the newest published release to
// component's image reference. Every release here is created with
// --prerelease, so GitHub's /releases/latest (which excludes prereleases)
// can't be used - the full list is queried instead and ranked locally (see
// pickLatestReleaseImage).
func ResolveLatestReleaseImage(ctx context.Context, client *http.Client, component string) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/foxly-it/rootguard/releases", nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("query GitHub releases: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub releases API returned %s", response.Status)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", fmt.Errorf("read GitHub releases response: %w", err)
	}
	return pickLatestReleaseImage(body, component)
}

// resolveTargetImage returns spec.ResolveTarget's live-discovered image when
// set and successful, falling back to the static TargetImage pin otherwise -
// a transient GitHub Releases lookup failure degrades to today's
// static-pin behavior instead of blocking the check/update.
func resolveTargetImage(ctx context.Context, spec ServiceSpec) string {
	if spec.ResolveTarget == nil {
		return spec.TargetImage
	}
	if image, err := spec.ResolveTarget(ctx); err == nil && image != "" {
		return image
	}
	return spec.TargetImage
}

// digestQualify turns a bare "repo:tag" reference - what live release
// discovery produces - into an immutable "repo@sha256:..." one, using the
// digest the image was just pulled at. Cosign attestation verification
// requires an explicit @sha256: reference and reports "not_applicable"
// without one, same as it already does for any other mutable-tag image;
// an already-qualified (static pin) target passes through unchanged, and a
// lookup failure falls back to the original reference rather than failing
// the check/update over a cosmetic gap.
func digestQualify(ctx context.Context, run CommandRunner, image string) string {
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
// one digest recorded against a matching local image, the first-match loop
// there can silently return a stale one instead of the one just pulled.
// Found via a real CI failure in rootguard-updater's own copy of this exact
// pattern; preferred here over digestQualify whenever it can parse pull's
// own output, with digestQualify kept as the fallback for an
// already-qualified static pin or an unexpected output format.
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
