package unbound

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPreviewReportsChangesWithoutWriting(t *testing.T) {
	manager := newTestManager(t)
	settings := DefaultSettings()
	settings.Threads = 4

	preview, err := manager.Preview(settings)
	if err != nil {
		t.Fatal(err)
	}
	if !preview.Changed || len(preview.Changes) != 1 || preview.Changes[0].Field != "threads" {
		t.Fatalf("unexpected preview: %+v", preview)
	}
	if !strings.Contains(preview.RenderedConfig, "num-threads: 4") {
		t.Fatalf("preview did not render proposed config: %s", preview.RenderedConfig)
	}
	if _, err := os.Stat(filepath.Join(manager.hostConfigDir, "settings.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("preview wrote settings: %v", err)
	}
}

func TestPreviewReportsPrivateDomainAndReversePolicyChanges(t *testing.T) {
	manager := newTestManager(t)
	settings := DefaultSettings()
	settings.PrivateDomains = []string{"home.example."}
	settings.ReverseZones = []ReverseZonePolicy{{Network: "192.168.0.0/16", Mode: reverseModeTransparent}}

	preview, err := manager.Preview(settings)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Changes) != 2 ||
		preview.Changes[0].Field != "private_domains" ||
		preview.Changes[1].Field != "reverse_zones" {
		t.Fatalf("unexpected private network preview: %+v", preview)
	}
}

func TestPreviewReportsLocalZoneChanges(t *testing.T) {
	manager := newTestManager(t)
	settings := DefaultSettings()
	settings.LocalZones = []LocalZone{{
		Name:  "home.lab.",
		Hosts: []LocalHost{{Hostname: "printer", IPv4: "192.168.1.20", PTR: true}},
	}}
	preview, err := manager.Preview(settings)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Changes) != 1 || preview.Changes[0].Field != "local_zones" {
		t.Fatalf("unexpected local zone preview: %+v", preview)
	}
	if !strings.Contains(preview.RenderedConfig, `local-data-ptr: "192.168.1.20 printer.home.lab"`) {
		t.Fatalf("preview did not render the proposed local host: %s", preview.RenderedConfig)
	}
}

func TestPreviewReportsNetworkModeChange(t *testing.T) {
	manager := newTestManager(t)
	settings := DefaultSettings()
	settings.NetworkMode = networkModeDual
	preview, err := manager.Preview(settings)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Changes) != 1 || preview.Changes[0].Field != "network_mode" {
		t.Fatalf("unexpected network mode preview: %+v", preview)
	}
}

func TestApplyCreatesHistoryAndRestore(t *testing.T) {
	manager := newTestManager(t)
	settings := DefaultSettings()
	settings.Threads = 4
	if err := manager.Apply(context.Background(), settings); err != nil {
		t.Fatal(err)
	}

	history, err := manager.History()
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 {
		t.Fatalf("expected baseline and active versions, got %d", len(history))
	}
	var baselineID string
	for _, entry := range history {
		if settingsEqual(entry.Settings, DefaultSettings()) {
			baselineID = entry.ID
		}
	}
	if baselineID == "" {
		t.Fatal("default baseline was not recorded")
	}

	restored, err := manager.Restore(context.Background(), baselineID)
	if err != nil {
		t.Fatal(err)
	}
	if !settingsEqual(restored, DefaultSettings()) {
		t.Fatalf("unexpected restored settings: %+v", restored)
	}
	loaded, err := manager.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !settingsEqual(loaded, DefaultSettings()) {
		t.Fatalf("restore did not activate baseline: %+v", loaded)
	}
}

func TestApplyRestoresPreviousFilesWhenRestartFails(t *testing.T) {
	manager := newTestManager(t)
	initial := DefaultSettings()
	initial.Threads = 3
	if err := manager.Apply(context.Background(), initial); err != nil {
		t.Fatal(err)
	}

	restartCalls := 0
	manager.run = func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "restart" {
			restartCalls++
			if restartCalls == 1 {
				return []byte("restart failed"), errors.New("exit 1")
			}
		}
		return []byte("OK"), nil
	}
	changed := initial
	changed.Threads = 8
	if err := manager.Apply(context.Background(), changed); err == nil || !strings.Contains(err.Error(), "previous configuration restored") {
		t.Fatalf("expected a successful automatic rollback, got %v", err)
	}
	loaded, err := manager.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !settingsEqual(loaded, initial) {
		t.Fatalf("failed restart left changed settings active: %+v", loaded)
	}
	config, err := os.ReadFile(filepath.Join(manager.hostConfigDir, "50-rootguard.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(config), "num-threads: 3") {
		t.Fatalf("failed restart left changed config active: %s", config)
	}
}

func TestDiagnosticsChecksConfigurationResolutionAndDNSSEC(t *testing.T) {
	manager := newTestManager(t)
	manager.run = func(_ context.Context, _ string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "unbound-checkconf"):
			return []byte("unbound-checkconf: no errors"), nil
		case strings.Contains(joined, "example.com"):
			return []byte("93.184.216.34\n"), nil
		case strings.Contains(joined, "dnssec-failed.org"):
			return []byte(";; ->>HEADER<<- opcode: QUERY, status: SERVFAIL, id: 1"), nil
		default:
			return nil, errors.New("unexpected command")
		}
	}
	report := manager.Diagnose(context.Background())
	if !report.Healthy || len(report.Checks) != 3 {
		t.Fatalf("unexpected diagnostic report: %+v", report)
	}
}

func TestRestoreRejectsTraversal(t *testing.T) {
	manager := newTestManager(t)
	if _, err := manager.Restore(context.Background(), "../settings"); !errors.Is(err, ErrVersionNotFound) {
		t.Fatalf("expected version not found, got %v", err)
	}
}

func TestHistoryKeepsMostRecentTwentyVersions(t *testing.T) {
	manager := newTestManager(t)
	for threads := 1; threads <= 25; threads++ {
		settings := DefaultSettings()
		settings.Threads = threads
		if err := manager.Apply(context.Background(), settings); err != nil {
			t.Fatal(err)
		}
	}
	history, err := manager.History()
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != historyLimit {
		t.Fatalf("expected %d retained versions, got %d", historyLimit, len(history))
	}
	if history[0].Settings.Threads != 25 {
		t.Fatalf("latest version was not retained: %+v", history[0])
	}
}

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	manager := NewManager(t.TempDir(), "/etc/unbound/unbound.d", "rootguard-unbound")
	manager.run = func(_ context.Context, _ string, _ ...string) ([]byte, error) { return []byte("OK"), nil }
	base := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	calls := 0
	manager.now = func() time.Time {
		calls++
		return base.Add(time.Duration(calls) * time.Nanosecond)
	}
	return manager
}
