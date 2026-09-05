package main

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestDecodeTargetOverridesHandlesMissingAndPresentBody(t *testing.T) {
	overrides, err := decodeTargetOverrides(strings.NewReader(""))
	if err != nil || overrides != nil {
		t.Fatalf("expected no overrides for an empty body, got %#v, err=%v", overrides, err)
	}

	overrides, err = decodeTargetOverrides(strings.NewReader(`{"target_images":{"core":"core:resolved"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if overrides["core"] != "core:resolved" {
		t.Fatalf("unexpected overrides: %#v", overrides)
	}
}

func TestDecodeTargetOverridesRejectsInvalidJSON(t *testing.T) {
	if _, err := decodeTargetOverrides(strings.NewReader("{not json")); err == nil {
		t.Fatal("expected an error for invalid JSON")
	}
}

// TestControlPlaneCheckAppliesTargetImageOverride guards against check()
// silently ignoring a caller-supplied target image override in favor of
// the static TargetImage pin: the static pin here points at an image the
// fake docker runner does not recognize, so the check only succeeds if
// the override is actually what gets pulled/inspected and reported.
func TestControlPlaneCheckAppliesTargetImageOverride(t *testing.T) {
	current := map[string]string{"rootguard-core": "sha256:core-old"}
	candidates := map[string]string{"core:resolved": "sha256:core-new"}
	run := func(_ context.Context, arguments ...string) ([]byte, error) {
		switch {
		case arguments[0] == "inspect":
			container := arguments[len(arguments)-1]
			return []byte(container + ":old|" + current[container]), nil
		case arguments[0] == "image":
			image := arguments[len(arguments)-1]
			id, ok := candidates[image]
			if !ok {
				t.Fatalf("unexpected image inspected (override not applied): %q", image)
			}
			return []byte(id), nil
		case arguments[0] == "pull":
			image := arguments[len(arguments)-1]
			if image != "core:resolved" {
				t.Fatalf("unexpected image pulled (override not applied): %q", image)
			}
			return []byte("ok"), nil
		default:
			t.Fatalf("unexpected docker command: %v", arguments)
			return nil, nil
		}
	}
	specs := []serviceSpec{{
		Name: "core", DisplayName: "Core", Container: "rootguard-core",
		TargetImage: "core:static-pin-should-not-be-used",
		HealthURL:   "http://core/health",
	}}
	manager := newManager(t.TempDir(), "/compose.yaml", "rootguard", specs, run)
	if _, err := manager.StartCheck(map[string]string{"core": "core:resolved"}); err != nil {
		t.Fatal(err)
	}
	result := waitForState(t, manager, stateIdle)
	if len(result.Services) != 1 {
		t.Fatalf("expected one service result, got %#v", result.Services)
	}
	service := result.Services[0]
	if service.TargetImage != "core:resolved" {
		t.Fatalf("expected the override target image to be reported, got %q", service.TargetImage)
	}
	if !service.UpdateAvailable || service.CandidateID != "sha256:core-new" {
		t.Fatalf("expected an available update resolved via the override: %#v", service)
	}
}

// TestControlPlaneCheckFallsBackToStaticPinWithoutOverride is the inverse
// of the above: no override for a service means today's exact static-pin
// behavior, unaffected by the override mechanism's existence.
func TestControlPlaneCheckFallsBackToStaticPinWithoutOverride(t *testing.T) {
	run := func(_ context.Context, arguments ...string) ([]byte, error) {
		switch arguments[0] {
		case "inspect":
			return []byte("rootguard-core:old|sha256:core-old"), nil
		case "pull":
			if arguments[len(arguments)-1] != "core:static-pin" {
				t.Fatalf("expected the static pin to be pulled, got %v", arguments)
			}
			return []byte("ok"), nil
		case "image":
			return []byte("sha256:core-old"), nil
		default:
			t.Fatalf("unexpected docker command: %v", arguments)
			return nil, nil
		}
	}
	specs := []serviceSpec{{
		Name: "core", DisplayName: "Core", Container: "rootguard-core",
		TargetImage: "core:static-pin",
		HealthURL:   "http://core/health",
	}}
	manager := newManager(t.TempDir(), "/compose.yaml", "rootguard", specs, run)
	if _, err := manager.StartCheck(map[string]string{"webapp": "webapp:resolved"}); err != nil {
		t.Fatal(err)
	}
	result := waitForState(t, manager, stateIdle)
	if result.Services[0].TargetImage != "core:static-pin" {
		t.Fatalf("expected the static pin unaffected, got %q", result.Services[0].TargetImage)
	}
}

// TestControlPlaneRejectsOverrideOutsidePinnedRepository is the regression
// test for a real gap found in review: target_images used to reach
// `docker pull` completely unchecked, letting a holder of
// ROOTGUARD_UPDATER_TOKEN force the host dockerd to pull from any
// registry - undermining the internet isolation the attestation-proxy +
// `internal: true` control network are meant to guarantee. Covers both a
// completely different repository and a same-prefix sibling repository
// (e.g. "rootguard-core-evil"), and confirms docker is never invoked at
// all once the allowlist check has already failed.
func TestControlPlaneRejectsOverrideOutsidePinnedRepository(t *testing.T) {
	run := func(_ context.Context, arguments ...string) ([]byte, error) {
		t.Fatalf("docker must never be invoked when the override fails the allowlist check, got %v", arguments)
		return nil, nil
	}
	specs := []serviceSpec{{
		Name: "core", DisplayName: "Core", Container: "rootguard-core",
		TargetImage: "ghcr.io/foxly-it/rootguard-core:latest",
		HealthURL:   "http://core/health",
	}}
	for _, override := range []string{
		"ghcr.io/attacker/evil-image:latest",
		"ghcr.io/foxly-it/rootguard-core-evil:latest",
	} {
		manager := newManager(t.TempDir(), "/compose.yaml", "rootguard", specs, run)
		if _, err := manager.StartCheck(map[string]string{"core": override}); !errors.Is(err, errTargetOverrideNotAllowlisted) {
			t.Fatalf("StartCheck: expected errTargetOverrideNotAllowlisted for %q, got %v", override, err)
		}
		if _, err := manager.StartUpdate(map[string]string{"core": override}); !errors.Is(err, errTargetOverrideNotAllowlisted) {
			t.Fatalf("StartUpdate: expected errTargetOverrideNotAllowlisted for %q, got %v", override, err)
		}
	}
}

// TestDigestQualifyAttachesTheRealDigest guards against a live-resolved
// bare-tag override (from Core's GitHub Releases discovery) staying
// tag-only in the reported/recorded image after a successful pull -
// cosign attestation requires an explicit @sha256: reference and reports
// "not_applicable" without one.
func TestDigestQualifyAttachesTheRealDigest(t *testing.T) {
	run := func(_ context.Context, arguments ...string) ([]byte, error) {
		if arguments[0] != "image" || arguments[1] != "inspect" {
			t.Fatalf("unexpected command: %v", arguments)
		}
		return []byte("ghcr.io/foxly-it/rootguard-core@sha256:deadbeef|"), nil
	}
	image := digestQualify(context.Background(), run, "ghcr.io/foxly-it/rootguard-core:0.1.0-beta.5")
	if image != "ghcr.io/foxly-it/rootguard-core@sha256:deadbeef" {
		t.Fatalf("expected the digest-qualified reference, got %q", image)
	}
}

func TestDigestQualifyLeavesAlreadyQualifiedImagesUnchanged(t *testing.T) {
	run := func(context.Context, ...string) ([]byte, error) {
		t.Fatal("should not inspect an already digest-qualified image")
		return nil, nil
	}
	const image = "ghcr.io/foxly-it/rootguard-core:0.1.0-beta.5@sha256:cafe"
	if got := digestQualify(context.Background(), run, image); got != image {
		t.Fatalf("expected the image unchanged, got %q", got)
	}
}
