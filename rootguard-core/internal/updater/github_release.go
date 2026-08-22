package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

// releaseTagPattern matches RootGuard's own release-tag convention, the same
// pattern release-alpha.yml's own "version" job validates new tags against.
var releaseTagPattern = regexp.MustCompile(`^v0\.1\.0-(alpha|beta)\.[0-9]+$`)

type githubRelease struct {
	TagName string `json:"tag_name"`
}

// pickLatestReleaseImage scans a GitHub Releases API response body
// (newest-first, GitHub's own list order) for the newest RootGuard release
// tag and returns the matching ghcr.io image reference for component.
func pickLatestReleaseImage(body []byte, component string) (string, error) {
	var releases []githubRelease
	if err := json.Unmarshal(body, &releases); err != nil {
		return "", fmt.Errorf("parse GitHub releases response: %w", err)
	}
	for _, release := range releases {
		if releaseTagPattern.MatchString(release.TagName) {
			version := strings.TrimPrefix(release.TagName, "v")
			return fmt.Sprintf("ghcr.io/foxly-it/rootguard-%s:%s", component, version), nil
		}
	}
	return "", fmt.Errorf("no matching RootGuard release found")
}

// ResolveLatestReleaseImage queries the public GitHub Releases API for
// foxly-it/rootguard and resolves the newest published release to
// component's image reference. Every release here is created with
// --prerelease, so GitHub's /releases/latest (which excludes prereleases)
// can't be used - the full, newest-first list is queried instead.
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
