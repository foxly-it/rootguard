package updater

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/mod/semver"
)

const (
	releasesURL = "https://api.github.com/repos/foxly-it/rootguard/releases"
	// releaseCacheTTL is how long one successful lookup serves every
	// component. Long enough that a check and the install that follows it
	// see the same release; short enough that a new release shows up
	// without restarting Core.
	releaseCacheTTL = 10 * time.Minute
	// releaseRetryAfter keeps a failed lookup from being repeated for every
	// component of the same check round.
	releaseRetryAfter = time.Minute
	// maxReleasesBody bounds the response read (30 releases with notes are
	// well under 1 MiB).
	maxReleasesBody = 8 << 20
)

type githubRelease struct {
	TagName string `json:"tag_name"`
}

// pickLatestReleaseTag scans a GitHub Releases API response body for the
// newest RootGuard release tag. Ranks every valid semantic-version tag itself
// via golang.org/x/mod/semver rather than trusting the API response's own
// ordering - found via a real CI failure: querying the live API directly
// showed v0.1.0-beta.9 listed *ahead of* v0.1.0-beta.12, on a repository
// that had several releases cut in quick succession, so "the full,
// newest-first list" this function's doc comment used to promise isn't
// actually guaranteed. Using full SemVer precedence instead of a
// RootGuard-specific "0.1.0-(alpha|beta).N" parser also means this keeps
// working correctly across a future base-version change (0.2.0,
// 1.0.0-rc.1, a bare 1.0.0, ...) without needing another fix the day the
// scheme changes - mirrors the same choice made for rootguard-updater's
// own isOlderReleaseVersion (a different Go module, so its own copy).
// release.TagName is used as-is (GitHub returns it with the leading "v",
// e.g. "v0.1.0-beta.14" - exactly what semver.IsValid/Compare expect).
func pickLatestReleaseTag(body []byte) (string, error) {
	var releases []githubRelease
	if err := json.Unmarshal(body, &releases); err != nil {
		return "", fmt.Errorf("parse GitHub releases response: %w", err)
	}
	var bestTag string
	for _, release := range releases {
		if !semver.IsValid(release.TagName) {
			continue
		}
		if bestTag == "" || semver.Compare(release.TagName, bestTag) > 0 {
			bestTag = release.TagName
		}
	}
	if bestTag == "" {
		return "", fmt.Errorf("no matching RootGuard release found")
	}
	return bestTag, nil
}

// releaseImage is component's image reference in the release tagged tag
// ("v1.0.14" -> "ghcr.io/foxly-it/rootguard-core:1.0.14").
func releaseImage(component, tag string) string {
	return fmt.Sprintf("ghcr.io/foxly-it/rootguard-%s:%s", component, strings.TrimPrefix(tag, "v"))
}

// ReleaseResolver finds the newest published RootGuard release on GitHub and
// derives every component's update target from that one answer.
//
// Found live, 2026-11-13 (upgrade journey 1.0.13 -> 1.0.14): every component
// used to query GitHub on its own, unauthenticated, on every check and every
// update - a dozen lookups for one round of updates, against a limit of 60 an
// hour per IP that GitHub-hosted runners and households behind carrier-grade
// NAT share. When the second of Core's and WebApp's lookups failed, Core moved
// to 1.0.14 while WebApp kept the installed 1.0.13 pin, the update still
// reported success, and every check after it silently offered nothing.
//
// One lookup now serves all components for releaseCacheTTL, so they always
// agree on the release, and a check and the install after it see the same
// one. Refreshes are conditional requests with the last ETag, which GitHub
// doesn't count against the limit when nothing changed. A failed refresh keeps
// the last release seen instead of dropping back to the install-day pins, and
// is not repeated for releaseRetryAfter.
//
// The full release list is ranked locally rather than asking for
// /releases/latest, see pickLatestReleaseTag.
type ReleaseResolver struct {
	client     *http.Client
	url        string
	ttl        time.Duration
	retryAfter time.Duration
	now        func() time.Time

	mu        sync.Mutex
	tag       string
	etag      string
	err       error
	checkedAt time.Time
}

func NewReleaseResolver(client *http.Client) *ReleaseResolver {
	return &ReleaseResolver{
		client:     client,
		url:        releasesURL,
		ttl:        releaseCacheTTL,
		retryAfter: releaseRetryAfter,
		now:        time.Now,
	}
}

// Image returns component's image reference in the newest published release.
func (r *ReleaseResolver) Image(ctx context.Context, component string) (string, error) {
	tag, err := r.latestTag(ctx)
	if err != nil {
		return "", err
	}
	return releaseImage(component, tag), nil
}

func (r *ReleaseResolver) latestTag(ctx context.Context) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.now()
	if r.err == nil && r.tag != "" && now.Sub(r.checkedAt) < r.ttl {
		return r.tag, nil
	}
	if r.err != nil && now.Sub(r.checkedAt) < r.retryAfter {
		return r.lastKnownLocked()
	}
	tag, err := r.fetchLocked(ctx)
	r.checkedAt = now
	r.err = err
	if err != nil {
		if r.tag != "" {
			log.Printf("GitHub release lookup failed, keeping the last known release %s: %v", r.tag, err)
		} else {
			log.Printf("GitHub release lookup failed, update checks fall back to the installed pins: %v", err)
		}
		return r.lastKnownLocked()
	}
	r.tag = tag
	return tag, nil
}

// lastKnownLocked answers from the last successful lookup when there was one,
// otherwise with the error of the failed one.
func (r *ReleaseResolver) lastKnownLocked() (string, error) {
	if r.tag != "" {
		return r.tag, nil
	}
	return "", r.err
}

func (r *ReleaseResolver) fetchLocked(ctx context.Context) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, r.url, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	if r.tag != "" && r.etag != "" {
		request.Header.Set("If-None-Match", r.etag)
	}
	response, err := r.client.Do(request)
	if err != nil {
		return "", fmt.Errorf("query GitHub releases: %w", err)
	}
	defer response.Body.Close()
	switch {
	case response.StatusCode == http.StatusNotModified && r.tag != "":
		return r.tag, nil
	case response.StatusCode != http.StatusOK:
		return "", releaseLookupError(response)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxReleasesBody))
	if err != nil {
		return "", fmt.Errorf("read GitHub releases response: %w", err)
	}
	tag, err := pickLatestReleaseTag(body)
	if err != nil {
		return "", err
	}
	r.etag = response.Header.Get("ETag")
	return tag, nil
}

// releaseLookupError names GitHub's rate limit explicitly - by far the most
// likely reason for a 403 or 429 here, and the one an operator can wait out.
func releaseLookupError(response *http.Response) error {
	if response.Header.Get("X-RateLimit-Remaining") == "0" {
		if reset, err := strconv.ParseInt(response.Header.Get("X-RateLimit-Reset"), 10, 64); err == nil {
			return fmt.Errorf("GitHub API rate limit reached until %s", time.Unix(reset, 0).UTC().Format("15:04 MST"))
		}
		return errors.New("GitHub API rate limit reached")
	}
	return fmt.Errorf("GitHub releases API returned %s", response.Status)
}

// resolveTargetImage returns spec.ResolveTarget's live-discovered image when
// set and successful. Otherwise it returns the static TargetImage pin, so a
// failed GitHub lookup degrades to comparing against the installed release
// instead of blocking the check or update, together with the reason, which
// the check reports in its status message.
func resolveTargetImage(ctx context.Context, spec ServiceSpec) (string, error) {
	if spec.ResolveTarget == nil {
		return spec.TargetImage, nil
	}
	image, err := spec.ResolveTarget(ctx)
	if err == nil && image == "" {
		err = errors.New("release lookup returned no image")
	}
	if err != nil {
		return spec.TargetImage, err
	}
	return image, nil
}

// NewerPinnedImage picks the update target for a third-party image, which has
// no RootGuard release to discover. configured comes from the installation's
// compose file or .env, which self-update never rewrites, so it stays at
// whatever was current on install day; shipped moves with every Core image.
// Without this, an installation would keep reporting that image as up to date
// forever. Only an older pin of the same repository gives way: a floating tag
// or another registry is the operator's own choice, and either way the update
// itself still starts by hand.
func NewerPinnedImage(configured, shipped string) string {
	configuredRepo, configuredVersion, ok := pinnedVersion(configured)
	if !ok {
		return configured
	}
	shippedRepo, shippedVersion, ok := pinnedVersion(shipped)
	if !ok || configuredRepo != shippedRepo {
		return configured
	}
	if semver.Compare(configuredVersion, shippedVersion) < 0 {
		return shipped
	}
	return configured
}

// pinnedVersion splits a "repo:vX.Y.Z[@sha256:...]" reference. ok is false
// for anything without a semantic version tag.
func pinnedVersion(image string) (repo, version string, ok bool) {
	reference, _, _ := strings.Cut(image, "@")
	repo, ok = imageRepo(reference)
	if !ok {
		return "", "", false
	}
	version = strings.TrimPrefix(reference, repo+":")
	return repo, version, semver.IsValid(version)
}

// digestQualify turns a bare "repo:tag" reference - what live release
// discovery produces - into an immutable "repo@sha256:..." one, using the
// digest the image was just pulled at. Cosign attestation verification
// requires an explicit @sha256: reference and reports "not_applicable"
// without one, same as it already does for any other mutable-tag image;
// an already-qualified (static pin) target passes through unchanged, and a
// lookup failure falls back to the original reference rather than failing
// the check/update over a cosmetic gap.
//
// Kept in sync by hand with rootguard-updater/image.go's identical copy
// (also digestQualify) - separate Go modules can't share an internal
// package, and standing up a third shared module was judged not worth it
// for ~30 lines of stable, rarely-changing logic (see this session's
// docs/project-state.md entry on the atomic-file-write consolidation for
// the same call made there). Check that file too if this one changes.
func digestQualify(ctx context.Context, run CommandRunner, image string) string {
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
// one digest recorded against a matching local image, the first-match loop
// there can silently return a stale one instead of the one just pulled.
// Found via a real CI failure in rootguard-updater's own copy of this exact
// pattern; preferred here over digestQualify whenever it can parse pull's
// own output, with digestQualify kept as the fallback for an
// already-qualified static pin or an unexpected output format.
//
// Also kept in sync by hand with rootguard-updater/image.go's identical
// copy - see digestQualify's own doc comment above for why.
//
// Found live via manual verification of the rootguard#525 fix (2026-09-08):
// an already-qualified image (e.g. AdGuard's static "repo:tag@sha256:..."
// pin, which never goes through live release discovery) reached this
// function unguarded. imageRepo's own colon-splitting only accounts for a
// tag separator, not a digest's internal colon, so it silently mis-split
// the reference and this function appended a second "@sha256:..." on top
// of the first, producing an invalid, doubled reference. digestQualify
// already special-cased this exact input shape; this function needed the
// identical guard.
func digestFromPullOutput(image string, output []byte) (string, bool) {
	if strings.Contains(image, "@sha256:") {
		return image, true
	}
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
// reference. Found in a follow-up review of internal/installer's
// identical, newer copy of this same lookup - the bug predates that copy
// and was already live here. A colon only separates the tag if it
// appears after the last "/"; any colon before that (or without a
// following "/" at all) is part of the registry host:port, not a tag
// separator - the same rule Docker's own reference parser
// (distribution/reference) uses. ok is false only when there's no colon
// after the repository path at all (an already-bare, tagless reference),
// matching the previous strings.Cut behavior's own "not found" case.
//
// Kept in sync by hand with rootguard-updater's identical copy - see
// digestQualify's own doc comment above for why.
func imageRepo(image string) (repo string, ok bool) {
	lastSlash := strings.LastIndex(image, "/")
	lastColon := strings.LastIndex(image, ":")
	if lastColon <= lastSlash {
		return "", false
	}
	return image[:lastColon], true
}
