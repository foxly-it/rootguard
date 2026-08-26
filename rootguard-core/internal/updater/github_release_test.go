package updater

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPickLatestReleaseImagePicksNewestMatchingTag(t *testing.T) {
	body := []byte(`[
		{"tag_name":"v0.1.0-beta.5"},
		{"tag_name":"not-a-rootguard-release"},
		{"tag_name":"v0.1.0-beta.4"},
		{"tag_name":"v0.1.0-alpha.16"}
	]`)
	image, err := pickLatestReleaseImage(body, "unbound")
	if err != nil {
		t.Fatal(err)
	}
	if image != "ghcr.io/foxly-it/rootguard-unbound:0.1.0-beta.5" {
		t.Fatalf("unexpected image: %q", image)
	}
}

// TestPickLatestReleaseImageIgnoresAPIResponseOrder is the regression test
// for a real bug: this function used to trust the GitHub Releases API's
// own list order as newest-first (its own doc comment promised it), but a
// direct query against the live API - on a repository that had several
// releases cut in quick succession - returned v0.1.0-beta.9 *ahead of*
// v0.1.0-beta.12. That silently made every self-update resolve to a much
// older release than the one actually just published. This body puts the
// true newest release last, matching what was observed live.
func TestPickLatestReleaseImageIgnoresAPIResponseOrder(t *testing.T) {
	body := []byte(`[
		{"tag_name":"v0.1.0-beta.9"},
		{"tag_name":"v0.1.0-beta.8"},
		{"tag_name":"v0.1.0-beta.12"},
		{"tag_name":"v0.1.0-beta.11"},
		{"tag_name":"v0.1.0-beta.10"}
	]`)
	image, err := pickLatestReleaseImage(body, "core")
	if err != nil {
		t.Fatal(err)
	}
	if image != "ghcr.io/foxly-it/rootguard-core:0.1.0-beta.12" {
		t.Fatalf("unexpected image: %q", image)
	}
}

// TestPickLatestReleaseImagePrefersBetaOverAlphaOnAHigherBuildNumber
// guards the series ranking: a beta build must always outrank every alpha
// build, even one with a numerically higher build number (the two series
// count independently, so e.g. alpha.20 existing doesn't mean it's newer
// than beta.1).
func TestPickLatestReleaseImagePrefersBetaOverAlphaOnAHigherBuildNumber(t *testing.T) {
	body := []byte(`[
		{"tag_name":"v0.1.0-alpha.20"},
		{"tag_name":"v0.1.0-beta.1"}
	]`)
	image, err := pickLatestReleaseImage(body, "core")
	if err != nil {
		t.Fatal(err)
	}
	if image != "ghcr.io/foxly-it/rootguard-core:0.1.0-beta.1" {
		t.Fatalf("expected the beta release to win over a higher-numbered alpha, got %q", image)
	}
}

// TestPickLatestReleaseImageAcrossABaseVersionChange guards that a future
// base-version bump correctly outranks every prerelease of the current
// 0.1.0 series, and that an rc prerelease correctly loses to the stable
// release it leads up to - the exact scenario a RootGuard-specific
// "0.1.0-(alpha|beta).N" parser couldn't have handled without yet another
// code change.
func TestPickLatestReleaseImageAcrossABaseVersionChange(t *testing.T) {
	body := []byte(`[
		{"tag_name":"v0.1.0-beta.14"},
		{"tag_name":"v0.2.0"},
		{"tag_name":"v1.0.0-rc.1"}
	]`)
	image, err := pickLatestReleaseImage(body, "core")
	if err != nil {
		t.Fatal(err)
	}
	if image != "ghcr.io/foxly-it/rootguard-core:1.0.0-rc.1" {
		t.Fatalf("expected the rc prerelease to outrank both 0.2.0 and 0.1.0-beta.14, got %q", image)
	}

	stable := []byte(`[
		{"tag_name":"v1.0.0-rc.1"},
		{"tag_name":"v1.0.0"}
	]`)
	image, err = pickLatestReleaseImage(stable, "core")
	if err != nil {
		t.Fatal(err)
	}
	if image != "ghcr.io/foxly-it/rootguard-core:1.0.0" {
		t.Fatalf("expected the stable release to outrank its own rc, got %q", image)
	}
}

// TestPickLatestReleaseImageAcceptsAnyValidSemver guards the intentional
// generalization: v9.9.9 doesn't match RootGuard's own "0.1.0-*" scheme,
// but it's a perfectly valid semantic version, and a future base-version
// change is exactly what this ranking is meant to survive without another
// code change - it must not be rejected just because it doesn't look like
// today's convention.
func TestPickLatestReleaseImageAcceptsAnyValidSemver(t *testing.T) {
	image, err := pickLatestReleaseImage([]byte(`[{"tag_name":"v9.9.9"}]`), "unbound")
	if err != nil {
		t.Fatal(err)
	}
	if image != "ghcr.io/foxly-it/rootguard-unbound:9.9.9" {
		t.Fatalf("unexpected image: %q", image)
	}
}

func TestPickLatestReleaseImageRejectsUnmatchedList(t *testing.T) {
	if _, err := pickLatestReleaseImage([]byte(`[{"tag_name":"not-a-version"},{"tag_name":"release-42"}]`), "unbound"); err == nil {
		t.Fatal("expected an error for a release list with no valid semantic version tag")
	}
}

// TestCheckUsesResolveTargetWhenSet guards against check() silently falling
// back to the static TargetImage pin instead of calling a configured
// ResolveTarget: the static pin here points at an image the fake docker
// runner does not recognize, so the check only succeeds if the resolver's
// return value is actually what gets pulled/inspected and reported.
func TestCheckUsesResolveTargetWhenSet(t *testing.T) {
	manager := NewManager(Options{
		DataDir: t.TempDir(), ComposeDir: t.TempDir(),
		Services: []ServiceSpec{{
			Name: "unbound", DisplayName: "Unbound", Container: "rootguard-unbound",
			TargetImage:   "unbound:static-pin-should-not-be-used",
			ResolveTarget: func(context.Context) (string, error) { return "unbound:resolved", nil },
		}},
		Run: func(_ context.Context, arguments ...string) ([]byte, error) {
			switch arguments[0] {
			case "inspect":
				return []byte("rootguard-unbound:v1|sha256:old"), nil
			case "pull":
				if arguments[len(arguments)-1] != "unbound:resolved" {
					t.Fatalf("unexpected image pulled (resolver not used): %v", arguments)
				}
				return []byte("ok"), nil
			case "image":
				if arguments[len(arguments)-1] != "unbound:resolved" {
					t.Fatalf("unexpected image inspected (resolver not used): %v", arguments)
				}
				return []byte("sha256:new"), nil
			default:
				t.Fatalf("unexpected docker command: %v", arguments)
				return nil, nil
			}
		},
	})

	if _, err := manager.StartCheck(); err != nil {
		t.Fatal(err)
	}
	waitForIdle(t, manager)
	service := manager.Status().Services[0]
	if service.TargetImage != "unbound:resolved" {
		t.Fatalf("expected the resolved target image to be reported, got %q", service.TargetImage)
	}
	if !service.UpdateAvailable || service.CandidateID != "sha256:new" {
		t.Fatalf("expected an available update resolved via ResolveTarget: %#v", service)
	}
}

// TestUpdateUsesResolveTargetWhenSet is the same guard as above but for the
// actual update(service) path, which independently re-reads spec.TargetImage
// at several call sites (pull, inspect, selectImage, the final recorded
// image) - each one needs to use the resolved value.
func TestUpdateUsesResolveTargetWhenSet(t *testing.T) {
	dataDir := t.TempDir()
	composeDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(composeDir, "compose.yaml"), []byte("services: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(Options{
		DataDir: dataDir, ComposeDir: composeDir,
		Services: []ServiceSpec{{
			Name: "unbound", DisplayName: "Unbound", Container: "rootguard-unbound",
			TargetImage:   "unbound:static-pin-should-not-be-used",
			ResolveTarget: func(context.Context) (string, error) { return "unbound:resolved", nil },
			BackupPaths:   []string{"/etc/unbound/unbound.d"},
		}},
		Run: func(_ context.Context, arguments ...string) ([]byte, error) {
			switch arguments[0] {
			case "inspect":
				return []byte("rootguard-unbound:v1|sha256:old"), nil
			case "pull":
				if arguments[len(arguments)-1] != "unbound:resolved" {
					t.Fatalf("unexpected image pulled (resolver not used): %v", arguments)
				}
				return []byte("ok"), nil
			case "image":
				if arguments[len(arguments)-1] != "unbound:resolved" {
					t.Fatalf("unexpected image inspected (resolver not used): %v", arguments)
				}
				return []byte("sha256:new"), nil
			default:
				return []byte("ok"), nil
			}
		},
		Verify: func(context.Context, string) error { return nil },
	})

	if _, err := manager.StartUpdate("unbound"); err != nil {
		t.Fatal(err)
	}
	waitForIdle(t, manager)
	result := manager.Status()
	if result.State != StateIdle {
		t.Fatalf("expected the update to succeed, got %#v", result)
	}
	if result.Services[0].CurrentImage != "unbound:resolved" {
		t.Fatalf("expected the recorded image to be the resolved one, got %#v", result.Services[0])
	}
}

// TestCheckFallsBackToStaticPinWhenResolveTargetFails guards against a
// transient GitHub Releases lookup failure (network issue, rate limit)
// blocking the whole check - it must degrade to today's static-pin
// behavior instead, since the actual running-container comparison is
// still meaningful without a live-discovered target.
func TestCheckFallsBackToStaticPinWhenResolveTargetFails(t *testing.T) {
	manager := NewManager(Options{
		DataDir: t.TempDir(), ComposeDir: t.TempDir(),
		Services: []ServiceSpec{{
			Name: "unbound", DisplayName: "Unbound", Container: "rootguard-unbound",
			TargetImage:   "unbound:static-pin",
			ResolveTarget: func(context.Context) (string, error) { return "", errors.New("boom") },
		}},
		Run: func(_ context.Context, arguments ...string) ([]byte, error) {
			switch arguments[0] {
			case "inspect":
				return []byte("rootguard-unbound:v1|sha256:old"), nil
			case "pull":
				if arguments[len(arguments)-1] != "unbound:static-pin" {
					t.Fatalf("expected the static pin to be pulled on resolve failure, got %v", arguments)
				}
				return []byte("ok"), nil
			case "image":
				return []byte("sha256:old"), nil
			default:
				t.Fatalf("unexpected docker command: %v", arguments)
				return nil, nil
			}
		},
	})

	if _, err := manager.StartCheck(); err != nil {
		t.Fatal(err)
	}
	waitForIdle(t, manager)
	service := manager.Status().Services[0]
	if service.Error != "" {
		t.Fatalf("expected no error - a resolve failure should fall back silently, got %#v", service)
	}
	if service.TargetImage != "unbound:static-pin" {
		t.Fatalf("expected the static pin to be reported, got %q", service.TargetImage)
	}
}

// TestDigestQualifyAttachesTheRealDigest guards against live-discovered
// bare-tag targets (e.g. "ghcr.io/foxly-it/rootguard-unbound:0.1.0-beta.5")
// staying tag-only after a successful pull - cosign attestation requires
// an explicit @sha256: reference and reports "not_applicable" without
// one, exactly the gap found live: a real self-update left the running
// container's image bare-tag-only even though the digest was available
// locally right after the pull.
func TestDigestQualifyAttachesTheRealDigest(t *testing.T) {
	run := func(_ context.Context, arguments ...string) ([]byte, error) {
		if arguments[0] != "image" || arguments[1] != "inspect" {
			t.Fatalf("unexpected command: %v", arguments)
		}
		return []byte("ghcr.io/foxly-it/rootguard-unbound@sha256:deadbeef|"), nil
	}
	image := digestQualify(context.Background(), run, "ghcr.io/foxly-it/rootguard-unbound:0.1.0-beta.5")
	if image != "ghcr.io/foxly-it/rootguard-unbound@sha256:deadbeef" {
		t.Fatalf("expected the digest-qualified reference, got %q", image)
	}
}

func TestDigestQualifyLeavesAlreadyQualifiedImagesUnchanged(t *testing.T) {
	run := func(context.Context, ...string) ([]byte, error) {
		t.Fatal("should not inspect an already digest-qualified image")
		return nil, nil
	}
	const image = "ghcr.io/foxly-it/rootguard-unbound:0.1.0-beta.5@sha256:cafe"
	if got := digestQualify(context.Background(), run, image); got != image {
		t.Fatalf("expected the image unchanged, got %q", got)
	}
}

func TestDigestQualifyFallsBackToTheBareTagOnLookupFailure(t *testing.T) {
	run := func(context.Context, ...string) ([]byte, error) {
		return nil, errors.New("docker daemon unreachable")
	}
	const image = "ghcr.io/foxly-it/rootguard-unbound:0.1.0-beta.5"
	if got := digestQualify(context.Background(), run, image); got != image {
		t.Fatalf("expected the bare tag as a fallback, got %q", got)
	}
}

// TestDigestFromPullOutputParsesARealPullTranscript is the regression test
// for a real bug caught live in CI: digestQualify's RepoDigests lookup
// returned a *stale* digest (an older release's, not the one just pulled)
// because RepoDigests belongs to the whole local image object, not to this
// specific pull. digestFromPullOutput reads the digest docker pull's own
// output reports instead, which can only ever be what was just pulled.
func TestDigestFromPullOutputParsesARealPullTranscript(t *testing.T) {
	output := []byte("0.1.0-beta.12: Pulling from foxly-it/rootguard-core\n" +
		"Digest: sha256:8cf049d4dffc1b3d456da8e89f92f089d74d4c85ec5cf8675ae1a0c88720f216\n" +
		"Status: Downloaded newer image for ghcr.io/foxly-it/rootguard-core:0.1.0-beta.12\n")
	got, ok := digestFromPullOutput("ghcr.io/foxly-it/rootguard-core:0.1.0-beta.12", output)
	if !ok {
		t.Fatal("expected a digest to be found")
	}
	want := "ghcr.io/foxly-it/rootguard-core@sha256:8cf049d4dffc1b3d456da8e89f92f089d74d4c85ec5cf8675ae1a0c88720f216"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDigestFromPullOutputReturnsFalseWithoutADigestLine(t *testing.T) {
	if _, ok := digestFromPullOutput("repo:tag", []byte("some unexpected output\n")); ok {
		t.Fatal("expected no digest to be found")
	}
}
