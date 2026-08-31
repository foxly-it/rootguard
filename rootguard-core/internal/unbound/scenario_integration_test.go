//go:build integration

// Scenario tests exercise the real guided-settings render path
// (Settings.Render, exactly what a user's WebGUI configuration produces)
// against a real running rootguard-unbound container, verified with real
// dig queries - not just string-matching the rendered config. Requires
// Docker and a locally built "rootguard-unbound:test" image (see
// rootguard-unbound/Dockerfile, matching the image ci-unbound.yml builds).
// Run with: go test -tags integration ./internal/unbound/... -run TestScenario -v
package unbound

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

const (
	scenarioImage     = "rootguard-unbound:test"
	scenarioContainer = "rootguard-unbound-scenario-ci"
	// dnssecTestZoneDir must match setup.sh/inject.sh's own OUT_DIR
	// default - DNSSEC_TEST_ZONE_DIR isn't set in the environments this
	// package's tests actually run in (ci-unbound.yml's "scenario-tests"
	// job, or a developer running setup.sh by hand), so both scripts
	// always fall back to this same path.
	dnssecTestZoneDir = "/tmp/rootguard-ci-dnssec-test"
)

func TestMain(m *testing.M) {
	// Only the scenario tests need a live container; every other test in
	// this package is a pure unit test and must keep working without
	// Docker at all, so the container lifecycle lives here rather than in
	// a package-level init that would run unconditionally.
	os.Exit(m.Run())
}

func startScenarioContainer(t *testing.T) {
	t.Helper()
	// Defensive: a container from an earlier interrupted run (a dropped
	// SSH session, a killed test binary) can outlive its own t.Cleanup,
	// and "docker run --name" fails outright against a stale survivor
	// rather than just reusing or replacing it.
	_, _ = exec.Command("docker", "rm", "-f", scenarioContainer).CombinedOutput()
	// Found in review: TestScenarioDNSSECFailures used to dig real
	// internet domains (dnssec-failed.org, example.com) directly, so a
	// transient DNS/network hiccup on the runner's own connection failed
	// this test for reasons that had nothing to do with the code under
	// test - confirmed live: main's own independent, scheduled run of
	// this exact test failed this way repeatedly. wireUpLocalDNSSECTestZone
	// below points it at ci.yml's own scripts/ci/dnssec-test-zone/setup.sh
	// authority instead, via the container's own network gateway.
	run(t, "docker", "run", "--rm", "--detach", "--name", scenarioContainer, scenarioImage)
	t.Cleanup(func() {
		_, _ = exec.Command("docker", "stop", scenarioContainer).CombinedOutput()
	})
	waitHealthy(t)
	wireUpLocalDNSSECTestZone(t)
}

// wireUpLocalDNSSECTestZone reuses the exact same inject.sh
// scripts/ci/dnssec-test-zone/setup.sh's own caller (ci-unbound.yml's
// "Test amd64"/"Test arm64" jobs) already uses and has verified live -
// one implementation of "find the local DNSSEC test authority from
// inside this specific container, then wire up forward-zone/
// trust-anchor", not a second Go reimplementation of it. Runs once,
// right after the container's initial startup, before any scenario's
// own config exists yet, so the restart inject.sh performs costs
// nothing extra here.
func wireUpLocalDNSSECTestZone(t *testing.T) {
	t.Helper()
	// go test's own working directory is always this package's directory
	// - three levels below the repository root.
	if output, err := exec.Command("../../../scripts/ci/dnssec-test-zone/inject.sh", scenarioContainer).CombinedOutput(); err != nil {
		t.Fatalf("wire up local DNSSEC test zone: %v: %s", err, output)
	}
	waitHealthy(t)
}

// localSplitDNSForwardTarget returns the split-DNS test authority
// container's own IP, as setup.sh started it and resolved it (running
// alongside the scenario container on Docker's default bridge, so it's
// directly reachable from inside it). Used as a guided ForwardZone
// target that only resolves setup.sh's unsigned
// split.rgtest-split.internal record - unlike rgtest-ci.internal, that
// authority is never forwarded by inject.sh's own base config, so a
// query for it only succeeds if the scenario's own ForwardZone setting
// actually took effect, not because of the CI harness's ambient wiring.
//
// A bare IP, no "@port" - found live: Settings.Render() calls
// Settings.Validate() first, which requires forward_zones[].servers[]
// to be a canonical IP address with no port suffix (that syntax is
// Unbound raw config's own forward-addr extension, which inject.sh's
// base wiring uses directly, not something the guided-settings API
// accepts). The split authority listens on the standard port 53 inside
// its own container specifically so this scenario - which drives the
// real Settings.Render() path, unlike inject.sh - can reach it with a
// production-shaped address; see setup.sh's own header comment for why
// that isn't the host's port 53.
func localSplitDNSForwardTarget(t *testing.T) string {
	t.Helper()
	authorityIP, err := os.ReadFile(dnssecTestZoneDir + "/split-authority-ip")
	if err != nil {
		t.Fatalf("read split-DNS test authority IP: %v", err)
	}
	return string(authorityIP)
}

func waitHealthy(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		output := run(t, "docker", "inspect", "--format", "{{.State.Health.Status}}", scenarioContainer)
		if strings.TrimSpace(output) == "healthy" {
			return
		}
		time.Sleep(time.Second)
	}
	t.Fatalf("%s never became healthy", scenarioContainer)
}

// applyScenario renders settings exactly the way a real deployment would
// (the same Settings.Render a user's guided configuration produces),
// writes it to the managed config path inside the running container, and
// reloads Unbound - the same file Manager.Apply manages in production
// (see the ManagedConfig path in ActiveConfiguration), just written
// directly here since this test drives the container without a Manager.
func applyScenario(t *testing.T, settings Settings) {
	t.Helper()
	rendered, err := settings.Render()
	if err != nil {
		t.Fatalf("render scenario settings: %v", err)
	}
	write := exec.Command("docker", "exec", "-i", scenarioContainer, "sh", "-c", "cat > /etc/unbound/unbound.d/50-rootguard.conf")
	write.Stdin = strings.NewReader(string(rendered))
	if output, err := write.CombinedOutput(); err != nil {
		t.Fatalf("write scenario config: %v: %s", err, output)
	}
	run(t, "docker", "exec", scenarioContainer, "unbound-checkconf")
	run(t, "docker", "exec", scenarioContainer, "unbound-control", "reload")
}

func dig(t *testing.T, args ...string) string {
	t.Helper()
	full := append([]string{"exec", scenarioContainer, "dig", "@127.0.0.1", "-p", "5335", "+time=5", "+tries=2"}, args...)
	return run(t, "docker", full...)
}

func run(t *testing.T, name string, args ...string) string {
	t.Helper()
	output, err := exec.CommandContext(context.Background(), name, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v: %s", name, strings.Join(args, " "), err, output)
	}
	return string(output)
}

func baseScenarioSettings() Settings {
	settings := DefaultSettings()
	settings.ForwardZones = []ForwardZone{}
	settings.LocalZones = []LocalZone{}
	return settings
}

// TestScenarioHomeNetwork covers the baseline case: a single local zone
// with a couple of hosts, PTR records, and the default RFC1918
// reverse-zone policy (NXDOMAIN for unassigned private addresses) still
// resolving normal external names recursively.
func TestScenarioHomeNetwork(t *testing.T) {
	startScenarioContainer(t)

	settings := baseScenarioSettings()
	settings.LocalZones = []LocalZone{{
		Name: "home.lab.",
		Hosts: []LocalHost{
			{Hostname: "router", IPv4: "192.168.1.1", PTR: true},
			{Hostname: "nas", IPv4: "192.168.1.10", PTR: true},
		},
	}}
	applyScenario(t, settings)

	if a := dig(t, "router.home.lab", "A", "+short"); strings.TrimSpace(a) != "192.168.1.1" {
		t.Fatalf("expected router.home.lab A to be 192.168.1.1, got %q", a)
	}
	if ptr := dig(t, "-x", "192.168.1.10", "+short"); !strings.Contains(ptr, "nas.home.lab.") {
		t.Fatalf("expected PTR for 192.168.1.10 to resolve to nas.home.lab., got %q", ptr)
	}
	// good.rgtest-ci.internal, not a real internet domain - see setup.sh's
	// own doc comment on why a blocking CI gate shouldn't depend on real
	// internet DNS being reachable and stable from whatever runner
	// happens to execute this test.
	if external := dig(t, "good.rgtest-ci.internal", "A", "+short"); strings.TrimSpace(external) == "" {
		t.Fatal("expected good.rgtest-ci.internal to resolve via normal recursion")
	}
	// The default reverse-zone policy is NXDOMAIN for unassigned RFC1918
	// addresses - an address in the same /24 but never registered as a
	// host must not leak an answer.
	if status := dig(t, "-x", "192.168.1.250", "+noall", "+comments"); !strings.Contains(status, "NXDOMAIN") {
		t.Fatalf("expected NXDOMAIN for an unassigned RFC1918 address, got %q", status)
	}
}

// TestScenarioVLANs covers several independent local zones (representing
// separate network segments) with the same host label in each, verifying
// they resolve to their own zone's address and never leak into another.
func TestScenarioVLANs(t *testing.T) {
	startScenarioContainer(t)

	// Hostnames must differ across zones too: RootGuard enforces global,
	// not per-zone, hostname uniqueness (see cross-zone hostname
	// uniqueness in settings.go) - a real, intentional guided-settings
	// constraint, so each segment's gateway gets its own distinct name.
	settings := baseScenarioSettings()
	settings.LocalZones = []LocalZone{
		{Name: "trusted.home.lab.", Hosts: []LocalHost{{Hostname: "trusted-gw", IPv4: "192.168.10.1"}}},
		{Name: "iot.home.lab.", Hosts: []LocalHost{{Hostname: "iot-gw", IPv4: "192.168.20.1"}}},
		{Name: "guest.home.lab.", Hosts: []LocalHost{{Hostname: "guest-gw", IPv4: "192.168.30.1"}}},
	}
	applyScenario(t, settings)

	cases := map[string]string{
		"trusted-gw.trusted.home.lab": "192.168.10.1",
		"iot-gw.iot.home.lab":         "192.168.20.1",
		"guest-gw.guest.home.lab":     "192.168.30.1",
	}
	for name, want := range cases {
		if got := strings.TrimSpace(dig(t, name, "A", "+short")); got != want {
			t.Fatalf("expected %s to resolve to %s, got %q", name, want, got)
		}
	}
}

// TestScenarioSplitDNS covers a forward zone routing one specific domain
// to its own dedicated resolver, with AllowUnsigned set the way a real
// private zone with no signing infrastructure of its own would need it -
// while everything else keeps using normal recursion. That's the core,
// provable split-DNS behavior: a named zone's queries take a distinct
// path from the default one, and the two don't interfere with each
// other. (An earlier version of this test tried to also prove
// AllowUnsigned's specific DNSSEC-relaxation effect by checking a real
// third-party domain's validation status - dropped: dnssec-failed.org is
// deliberately signed-with-broken-signatures rather than unsigned, so
// domain-insecure never suppressed that failure in the first place, and
// a domain this repo doesn't control staying in exactly the right DNSSEC
// state indefinitely wasn't a good bet regardless.)
//
// The forward target is setup.sh's own unsigned split-DNS zone, not a
// real internet domain - forwarding rgtest-ci.internal itself here
// wouldn't prove anything: inject.sh's own base config already forwards
// all of it before this scenario's settings are ever applied, so it
// would resolve identically whether or not Settings.Render's ForwardZone
// handling actually worked.
func TestScenarioSplitDNS(t *testing.T) {
	startScenarioContainer(t)

	settings := baseScenarioSettings()
	settings.ForwardZones = []ForwardZone{{
		Name:          "rgtest-split.internal.",
		Servers:       []string{localSplitDNSForwardTarget(t)},
		AllowUnsigned: true,
	}}
	applyScenario(t, settings)

	if got := strings.TrimSpace(dig(t, "split.rgtest-split.internal", "A", "+short")); got != "203.0.113.50" {
		t.Fatalf("expected the forwarded zone to resolve to 203.0.113.50, got %q", got)
	}
	// A domain outside that forward zone must still use normal recursion,
	// not the private upstream - the forward must not leak beyond its own
	// zone.
	if external := dig(t, "good.rgtest-ci.internal", "A", "+short"); strings.TrimSpace(external) == "" {
		t.Fatal("expected good.rgtest-ci.internal to still resolve via normal recursion outside the forward zone")
	}
}

// TestScenarioIPv6OnlyLocalRecords covers a local host with only an AAAA
// record and PTR generation, verifying no spurious A record exists and
// the ip6.arpa PTR resolves back correctly.
func TestScenarioIPv6OnlyLocalRecords(t *testing.T) {
	startScenarioContainer(t)

	settings := baseScenarioSettings()
	settings.NetworkMode = networkModeDual
	settings.LocalZones = []LocalZone{{
		Name:  "home.lab.",
		Hosts: []LocalHost{{Hostname: "printer", IPv6: "fd00::10", PTR: true}},
	}}
	applyScenario(t, settings)

	if aaaa := strings.TrimSpace(dig(t, "printer.home.lab", "AAAA", "+short")); aaaa != "fd00::10" {
		t.Fatalf("expected printer.home.lab AAAA to be fd00::10, got %q", aaaa)
	}
	if a := strings.TrimSpace(dig(t, "printer.home.lab", "A", "+short")); a != "" {
		t.Fatalf("expected no A record for an IPv6-only host, got %q", a)
	}
	if ptr := dig(t, "-x", "fd00::10", "+short"); !strings.Contains(ptr, "printer.home.lab.") {
		t.Fatalf("expected PTR for fd00::10 to resolve to printer.home.lab., got %q", ptr)
	}
}

// TestScenarioBrokenUpstream covers a forward zone pointing at an
// unreachable server - 192.0.2.1 is TEST-NET-1 (RFC 5737), reserved for
// documentation and guaranteed to never have a real listener. Uses an
// ordinary-looking name under a real TLD, not one of the RFC 6761
// special-use names (test/invalid/localhost/...) - Unbound answers those
// locally without ever consulting a forward-zone, which would make this
// pass for the wrong reason.
//
// This does not assert a fast SERVFAIL for the broken zone itself: an
// address that's silently dropped rather than actively refused (no TCP
// RST, no ICMP unreachable - true of most firewalled or unassigned
// ranges, including TEST-NET-1 in this environment) turns out to have no
// effective ceiling in Unbound's default retry/timeout behavior at all -
// empirically still pending after a full 5 minutes in manual testing.
// That's real resolver behavior, not a bug this test should fail on.
// What actually matters operationally is what this asserts instead: one
// permanently stuck forward zone must not block unrelated queries -
// verified by firing the broken zone's query in the background (deliberately
// not waited on) and confirming a normal domain outside that zone still
// resolves quickly.
func TestScenarioBrokenUpstream(t *testing.T) {
	startScenarioContainer(t)

	settings := baseScenarioSettings()
	settings.ForwardZones = []ForwardZone{{
		Name:    "broken.rootguard-ci-scenario.net.",
		Servers: []string{"192.0.2.1"},
	}}
	applyScenario(t, settings)

	go func() {
		_ = exec.Command("docker", "exec", scenarioContainer, "dig", "@127.0.0.1", "-p", "5335",
			"host.broken.rootguard-ci-scenario.net", "A", "+time=30", "+tries=1").Run()
	}()
	// Give the background query a head start so it's genuinely in flight
	// (past DNS lookup/connect, actively waiting on the unresponsive
	// forwarder) before the isolation check below starts timing itself.
	time.Sleep(2 * time.Second)

	start := time.Now()
	if external := dig(t, "good.rgtest-ci.internal", "A", "+short"); strings.TrimSpace(external) == "" {
		t.Fatal("expected normal recursion for an unrelated domain to be unaffected by the broken forward zone")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("expected an unrelated query to resolve quickly despite the stuck forward zone, took %s", elapsed)
	}
}

// TestScenarioDNSSECFailures closes the roadmap gap explicitly: the fixed
// base image's own DNSSEC check (ci-unbound.yml) only ever exercises the
// baked-in default config, never a real guided configuration. This
// applies a non-trivial one (local zones, an unrelated forward zone, a
// non-default resource profile) and verifies DNSSEC enforcement still
// holds for everything not explicitly overridden.
func TestScenarioDNSSECFailures(t *testing.T) {
	startScenarioContainer(t)

	settings := baseScenarioSettings()
	settings.ResourceProfile = resourceProfileSmall
	settings.LocalZones = []LocalZone{{
		Name:  "home.lab.",
		Hosts: []LocalHost{{Hostname: "router", IPv4: "192.168.1.1"}},
	}}
	// Never actually queried below - present only to prove DNSSEC
	// enforcement holds alongside an unrelated forward zone. 192.0.2.1 is
	// TEST-NET-1 (RFC 5737, reserved for documentation), same choice
	// TestScenarioBrokenUpstream already makes for the same reason: this
	// value must never resolve to a real service, since it's not
	// supposed to ever actually be dialed by this test.
	settings.ForwardZones = []ForwardZone{{Name: "example.org.", Servers: []string{"192.0.2.1"}}}
	applyScenario(t, settings)

	if status := dig(t, "bad.rgtest-ci.internal", "A", "+noall", "+comments"); !strings.Contains(status, "status: SERVFAIL") {
		t.Fatalf("expected bad.rgtest-ci.internal to SERVFAIL under a real guided configuration, got %q", status)
	}
	if status := dig(t, "good.rgtest-ci.internal", "A", "+dnssec", "+noall", "+comments"); !strings.Contains(status, " ad") {
		t.Fatalf("expected a validly-signed domain to still validate, got %q", status)
	}
	if a := strings.TrimSpace(dig(t, "router.home.lab", "A", "+short")); a != "192.168.1.1" {
		t.Fatalf("expected the local zone to still resolve alongside DNSSEC enforcement, got %q", a)
	}
}
