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

	restartCount := 0
	manager.run = func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "restart" {
			restartCount++
		}
		// Only the first restart's own readiness never arrives - the
		// rollback restart (restartCount == 2) must still succeed, so
		// this proves the rollback itself completing cleanly, not
		// compounding into a second readiness failure too (that's
		// TestApplyReportsWhenTheRollbackRestartItselfNeverBecomesReady).
		if restartCount <= 1 && len(args) >= 2 && args[0] == "exec" && args[len(args)-2] == "unbound-control" && args[len(args)-1] == "status" {
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

// TestWaitReadySkipsTheFinalSleep is the regression test for a small
// robustness finding: waitReady used to sleep unconditionally between
// every attempt, including after the very last one - a delay nothing
// afterward ever consumes, since the loop is about to give up and return
// an error either way. Counts sleep calls directly against a primary
// restart that never becomes ready (same setup as
// TestApplyRollsBackWhenUnboundNeverBecomesReady): must be exactly
// unboundReadyAttempts-1, one fewer than the number of attempts made.
func TestWaitReadySkipsTheFinalSleep(t *testing.T) {
	manager := newTestManager(t)
	initial := DefaultSettings()
	initial.Threads = 3
	if err := manager.Apply(context.Background(), initial); err != nil {
		t.Fatal(err)
	}

	sleepCalls := 0
	manager.sleep = func(time.Duration) <-chan time.Time {
		sleepCalls++
		fired := make(chan time.Time, 1)
		fired <- time.Time{}
		return fired
	}
	restartCount := 0
	manager.run = func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "restart" {
			restartCount++
		}
		// Only the first restart's readiness never arrives - the rollback
		// restart (restartCount == 2) succeeds on its very first check, so
		// it never sleeps at all and can't add to the count below.
		if restartCount <= 1 && len(args) >= 2 && args[0] == "exec" && args[len(args)-2] == "unbound-control" && args[len(args)-1] == "status" {
			return []byte("error: connect() failed"), errors.New("exit 1")
		}
		return []byte("OK"), nil
	}
	changed := initial
	changed.Threads = 8
	if err := manager.Apply(context.Background(), changed); err == nil {
		t.Fatal("expected Apply to fail against a daemon that never becomes ready")
	}
	if sleepCalls != unboundReadyAttempts-1 {
		t.Fatalf("expected exactly %d sleeps (one fewer than the attempt count, none after the last), got %d", unboundReadyAttempts-1, sleepCalls)
	}
}

// TestApplyReportsWhenTheRollbackRestartItselfNeverBecomesReady is the
// never-succeeds counterpart of the test above: if unbound-control status
// never comes back even for the *rollback* restart, rollbackFailedApply
// must say so honestly - not claim "previous configuration restored" when
// it never actually confirmed the restored config came up healthy.
func TestApplyReportsWhenTheRollbackRestartItselfNeverBecomesReady(t *testing.T) {
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
	if err == nil || strings.Contains(err.Error(), "previous configuration restored") || !strings.Contains(err.Error(), "rollback restart") {
		t.Fatalf("expected an honest report that the rollback restart's own readiness never arrived, got %v", err)
	}
}

// TestApplyRollbackUsesAFreshContextWhenTheOriginalWasCanceled is the
// regression test for a real gap found in review: rollbackFailedApply used
// to run the rollback restart (and, once waitReady existed, wait for it)
// with the *same* ctx Apply itself was called with. If that ctx is what's
// canceled - the realistic case waitReady's own ctx.Done() case exists for,
// e.g. the HTTP request that triggered Apply was aborted mid-flight - the
// rollback restart could be killed or refused to even start, leaving the
// just-restored *files* out of sync with whatever Unbound is actually still
// running. The rollback must detach from that cancellation instead.
func TestApplyRollbackUsesAFreshContextWhenTheOriginalWasCanceled(t *testing.T) {
	manager := newTestManager(t)
	initial := DefaultSettings()
	initial.Threads = 3
	if err := manager.Apply(context.Background(), initial); err != nil {
		t.Fatal(err)
	}

	restartCount := 0
	manager.run = func(ctx context.Context, _ string, args ...string) ([]byte, error) {
		if len(args) == 0 || args[0] != "restart" {
			return []byte("OK"), nil
		}
		restartCount++
		if restartCount == 1 {
			// The real first restart runs under the already-canceled ctx
			// Apply itself was called with - exec.CommandContext would
			// refuse to run a canceled context's command for real; here,
			// simulate that by failing exactly as a killed/refused
			// process would.
			if ctx.Err() == nil {
				t.Fatal("expected the first restart to observe the already-canceled input context")
			}
			return nil, ctx.Err()
		}
		// The rollback restart must not inherit that same cancellation.
		if ctx.Err() != nil {
			t.Fatal("rollback restart used a context that was still canceled - it must detach from the original request's cancellation")
		}
		return []byte("OK"), nil
	}

	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	changed := initial
	changed.Threads = 8
	err := manager.Apply(canceledCtx, changed)
	if err == nil || !strings.Contains(err.Error(), "previous configuration restored") {
		t.Fatalf("expected a successful rollback despite the canceled input context, got %v", err)
	}
	loaded, loadErr := manager.Load()
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if !settingsEqual(loaded, initial) {
		t.Fatalf("rollback under a canceled context left changed settings active: %+v", loaded)
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
