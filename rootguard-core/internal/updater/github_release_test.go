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

func TestPickLatestReleaseImageRejectsUnmatchedList(t *testing.T) {
	if _, err := pickLatestReleaseImage([]byte(`[{"tag_name":"v9.9.9"}]`), "unbound"); err == nil {
		t.Fatal("expected an error for a release list with no matching RootGuard tag")
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
