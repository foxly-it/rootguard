package unbound

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestExportReturnsCurrentSettingsAndCustomConfig(t *testing.T) {
	manager := newTestManager(t)
	settings := DefaultSettings()
	settings.Threads = 4
	if err := manager.Apply(context.Background(), settings); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.ApplyCustom(context.Background(), "server:\n    hide-identity: yes\n"); err != nil {
		t.Fatal(err)
	}

	bundle, err := manager.Export()
	if err != nil {
		t.Fatal(err)
	}
	if bundle.SchemaVersion != BundleSchemaVersion {
		t.Fatalf("unexpected schema version: %d", bundle.SchemaVersion)
	}
	if bundle.Settings.Threads != 4 {
		t.Fatalf("unexpected exported settings: %+v", bundle.Settings)
	}
	if bundle.CustomConfig != "server:\n    hide-identity: yes\n" {
		t.Fatalf("unexpected exported custom config: %q", bundle.CustomConfig)
	}
	if bundle.ExportedAt.IsZero() {
		t.Fatal("expected a non-zero export timestamp")
	}
}

func TestPreviewBundleReportsChangesWithoutWriting(t *testing.T) {
	manager := newTestManager(t)
	settings := DefaultSettings()
	settings.Threads = 4

	preview, err := manager.PreviewBundle(context.Background(), settings, "server:\n    hide-identity: yes\n")
	if err != nil {
		t.Fatal(err)
	}
	if !preview.Changed || len(preview.Changes) != 1 || preview.Changes[0].Field != "threads" {
		t.Fatalf("unexpected preview: %+v", preview)
	}
	if !preview.CustomChanged {
		t.Fatal("expected custom config to be reported as changed")
	}
	if _, err := os.Stat(filepath.Join(manager.hostConfigDir, "settings.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("preview wrote settings: %v", err)
	}
}

// A bundle import that changes settings and custom config together must be
// validated as the pair it will become, not against the stale value each
// side still has on disk - otherwise an import that resolves an existing
// guided/expert conflict by changing both sides at once would be rejected
// for a conflict that only existed in the old state.
func TestBundleValidatesSettingsAndCustomTogetherNotAgainstStaleState(t *testing.T) {
	manager := newTestManager(t)
	if _, err := manager.ApplyCustom(context.Background(), "forward-zone:\n    name: \"legacy.example.\"\n    forward-addr: 192.0.2.53\n"); err != nil {
		t.Fatal(err)
	}

	settings := DefaultSettings()
	settings.ForwardZones = []ForwardZone{{Name: "corp.example.", Servers: []string{"192.0.2.54"}}}

	// Plain Preview/Apply validate the new settings against the still-active
	// custom config and correctly reject this on its own.
	if _, err := manager.Preview(settings); !errors.Is(err, ErrInvalidCustomConfig) {
		t.Fatalf("expected guided/expert conflict from plain Preview, got %v", err)
	}

	// The same settings change, imported together with a custom config that
	// drops the conflicting forward-zone, must succeed.
	if _, err := manager.PreviewBundle(context.Background(), settings, ""); err != nil {
		t.Fatalf("expected bundle preview to accept the resolved pair, got %v", err)
	}
	if err := manager.ApplyBundle(context.Background(), settings, ""); err != nil {
		t.Fatalf("expected bundle apply to accept the resolved pair, got %v", err)
	}

	current, err := manager.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(current.ForwardZones) != 1 || current.ForwardZones[0].Name != "corp.example." {
		t.Fatalf("unexpected settings after bundle apply: %+v", current)
	}
	custom, err := manager.LoadCustom()
	if err != nil {
		t.Fatal(err)
	}
	if custom.Content != "" {
		t.Fatalf("expected custom config to be cleared, got %q", custom.Content)
	}
}

func TestApplyBundleRecordsHistoryAndIsRestorable(t *testing.T) {
	manager := newTestManager(t)
	settings := DefaultSettings()
	settings.Threads = 4
	if err := manager.ApplyBundle(context.Background(), settings, "server:\n    hide-identity: yes\n"); err != nil {
		t.Fatal(err)
	}

	history, err := manager.History()
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 {
		t.Fatalf("expected baseline and active versions, got %d", len(history))
	}
	if history[0].Settings.Threads != 4 || history[0].CustomConfig != "server:\n    hide-identity: yes\n" {
		t.Fatalf("unexpected latest version: %+v", history[0])
	}
}

func TestBundleRejectsInvalidSettings(t *testing.T) {
	manager := newTestManager(t)
	settings := DefaultSettings()
	settings.Threads = 0

	if _, err := manager.PreviewBundle(context.Background(), settings, ""); !errors.Is(err, ErrInvalidSettings) {
		t.Fatalf("expected invalid settings rejection, got %v", err)
	}
	if err := manager.ApplyBundle(context.Background(), settings, ""); !errors.Is(err, ErrInvalidSettings) {
		t.Fatalf("expected invalid settings rejection, got %v", err)
	}
}
