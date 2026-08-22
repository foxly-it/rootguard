package main

import (
	"context"
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
