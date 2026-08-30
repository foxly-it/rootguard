package unbound

import (
	"context"
	"errors"
	"fmt"
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

// TestApplyWaitsForUnboundToBecomeReadyAfterRestart is the regression test
// for a real race found live in CI: `docker restart` returning success only
// means the container process started again, not that Unbound has finished
// its own startup and opened its remote-control socket. A caller that
// immediately did anything else against the daemon (unbound-control,
// StartDiagnosticLogging's own verbosity call) could lose that race.
// Simulates unbound-control status failing for the first few polls, then
// succeeding - Apply must retry through that, not return prematurely.
func TestApplyWaitsForUnboundToBecomeReadyAfterRestart(t *testing.T) {
	manager := newTestManager(t)
	statusCalls := 0
	manager.run = func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "exec" && args[len(args)-2] == "unbound-control" && args[len(args)-1] == "status" {
			statusCalls++
			if statusCalls < 3 {
				return []byte("error: connect() failed"), errors.New("exit 1")
			}
		}
		return []byte("OK"), nil
	}
	if err := manager.Apply(context.Background(), DefaultSettings()); err != nil {
		t.Fatalf("expected Apply to retry through a slow-starting daemon, got %v", err)
	}
	if statusCalls != 3 {
		t.Fatalf("expected exactly 3 unbound-control status attempts (2 failures then success), got %d", statusCalls)
	}
}

// TestApplyWaitsForTheDNSPortToBecomeReadyAfterRestart is
// TestApplyWaitsForUnboundToBecomeReadyAfterRestart's sibling for the
// other half of waitReady: found in review, live, that unbound-control
// status succeeding doesn't guarantee the actual DNS listener has
// finished binding yet - a caller that queried it immediately afterward
// got a full "communications error ... timed out", not a slow response.
// Simulates unbound-control succeeding immediately but the DNS query
// itself failing for the first few polls.
func TestApplyWaitsForTheDNSPortToBecomeReadyAfterRestart(t *testing.T) {
	manager := newTestManager(t)
	digCalls := 0
	manager.run = func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if len(args) >= 3 && args[0] == "exec" && args[2] == "dig" {
			digCalls++
			if digCalls < 3 {
				return []byte(";; communications error to 127.0.0.1#5335: timed out"), errors.New("exit 9")
			}
		}
		return []byte("OK"), nil
	}
	if err := manager.Apply(context.Background(), DefaultSettings()); err != nil {
		t.Fatalf("expected Apply to retry through a slow-starting DNS listener, got %v", err)
	}
	if digCalls != 3 {
		t.Fatalf("expected exactly 3 DNS-readiness query attempts (2 failures then success), got %d", digCalls)
	}
}

// TestApplyRollsBackWhenUnboundNeverBecomesReady is
// TestApplyRestoresPreviousFilesWhenRestartFails' sibling for the new
// readiness wait: unbound-control status never succeeding is as real a
// sign the new config broke something as a restart command failing
// outright, and gets the same automatic rollback.
func TestApplyRollsBackWhenUnboundNeverBecomesReady(t *testing.T) {
	manager := newTestManager(t)
	initial := DefaultSettings()
	initial.Threads = 3
	if err := manager.Apply(context.Background(), initial); err != nil {
		t.Fatal(err)
	}

	manager.run = func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "exec" && args[len(args)-2] == "unbound-control" && args[len(args)-1] == "status" {
			return []byte("error: connect() failed"), errors.New("exit 1")
		}
		return []byte("OK"), nil
	}
	changed := initial
	changed.Threads = 8
	err := manager.Apply(context.Background(), changed)
	if err == nil || !strings.Contains(err.Error(), "wait for unbound to become ready") || !strings.Contains(err.Error(), "previous configuration restored") {
		t.Fatalf("expected a rollback citing the readiness timeout, got %v", err)
	}
	loaded, loadErr := manager.Load()
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if !settingsEqual(loaded, initial) {
		t.Fatalf("a never-ready restart left changed settings active: %+v", loaded)
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

// TestSetDiagnosticDomainsOverridesTheQueriedDomains is the regression
// test for the CI-only escape hatch main.go's own
// ROOTGUARD_UNBOUND_DIAGNOSTIC_RESOLUTION_DOMAIN/_DNSSEC_DOMAIN wire up -
// Diagnose must query whatever SetDiagnosticDomains was last given, not
// the hardcoded example.com/dnssec-failed.org defaults, and an empty
// argument must leave the corresponding domain untouched rather than
// clearing it.
func TestSetDiagnosticDomainsOverridesTheQueriedDomains(t *testing.T) {
	manager := newTestManager(t)
	manager.SetDiagnosticDomains("good.rgtest-ci.internal", "bad.rgtest-ci.internal")
	manager.run = func(_ context.Context, _ string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "unbound-checkconf"):
			return []byte("unbound-checkconf: no errors"), nil
		case strings.Contains(joined, "good.rgtest-ci.internal"):
			return []byte("203.0.113.10\n"), nil
		case strings.Contains(joined, "bad.rgtest-ci.internal"):
			return []byte(";; ->>HEADER<<- opcode: QUERY, status: SERVFAIL, id: 1"), nil
		default:
			return nil, fmt.Errorf("unexpected command querying the default domain instead of the overridden one: %s", joined)
		}
	}
	report := manager.Diagnose(context.Background())
	if !report.Healthy || len(report.Checks) != 3 {
		t.Fatalf("unexpected diagnostic report: %+v", report)
	}

	// An empty argument must not clear the domain SetDiagnosticDomains
	// was already given.
	manager.SetDiagnosticDomains("", "")
	report = manager.Diagnose(context.Background())
	if !report.Healthy {
		t.Fatalf("empty SetDiagnosticDomains arguments cleared a previously set domain: %+v", report)
	}
}

func TestDiagnosePathChecksResolutionAndDNSSECThroughAdGuard(t *testing.T) {
	manager := newTestManager(t)
	manager.run = func(_ context.Context, _ string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if !strings.Contains(joined, "@rootguard-adguard") || !strings.Contains(joined, "-p 53") {
			t.Fatalf("expected the check to target AdGuard's container address, got: %s", joined)
		}
		switch {
		case strings.Contains(joined, "example.com"):
			return []byte("93.184.216.34\n"), nil
		case strings.Contains(joined, "dnssec-failed.org"):
			return []byte(";; ->>HEADER<<- opcode: QUERY, status: SERVFAIL, id: 1"), nil
		default:
			return nil, errors.New("unexpected command")
		}
	}
	report := manager.DiagnosePath(context.Background(), "rootguard-adguard:53")
	if !report.Healthy || len(report.Checks) != 2 {
		t.Fatalf("unexpected path diagnostic report: %+v", report)
	}
	if report.Checks[0].Name != "adguard-resolution" || report.Checks[1].Name != "adguard-dnssec" {
		t.Fatalf("unexpected check names: %+v", report.Checks)
	}
}

func TestDiagnosePathFailsOpenOnInvalidAddress(t *testing.T) {
	manager := newTestManager(t)
	manager.run = func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		t.Fatal("dig should not run for an unparsable address")
		return nil, nil
	}
	report := manager.DiagnosePath(context.Background(), "not-a-valid-address")
	if report.Healthy {
		t.Fatalf("expected an unhealthy report for an invalid address, got: %+v", report)
	}
	for _, check := range report.Checks {
		if check.Passed {
			t.Fatalf("expected every check to fail closed: %+v", check)
		}
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
	// Fires immediately - waitReady's retry loop uses this between
	// polling attempts, and no test needs it to actually pause.
	manager.sleep = func(time.Duration) <-chan time.Time {
		fired := make(chan time.Time, 1)
		fired <- base
		return fired
	}
	return manager
}
