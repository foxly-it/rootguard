package unbound

import (
	"strings"
	"testing"
)

func findingFor(t *testing.T, findings []ImportFinding, directive string) ImportFinding {
	t.Helper()
	for _, f := range findings {
		if f.Directive == directive {
			return f
		}
	}
	t.Fatalf("no finding for directive %q in %+v", directive, findings)
	return ImportFinding{}
}

func TestImportAppliesScalarGuidedDirectives(t *testing.T) {
	content := "server:\n" +
		"    qname-minimisation: no\n" +
		"    prefetch: no\n" +
		"    prefetch-key: no\n" +
		"    aggressive-nsec: no\n" +
		"    serve-expired: no\n" +
		"    edns-buffer-size: 1480\n" +
		"    verbosity: 2\n" +
		"    serve-expired-ttl: 3600\n" +
		"    serve-expired-client-timeout: 900\n" +
		"    cache-min-ttl: 60\n" +
		"    cache-max-ttl: 43200\n" +
		"    num-threads: 8\n"

	result, err := ImportUnboundConf(DefaultSettings(), "", content)
	if err != nil {
		t.Fatal(err)
	}
	want := Settings{
		QnameMinimisation:         false,
		Prefetch:                  false,
		PrefetchKey:               false,
		AggressiveNSEC:            false,
		ServeExpired:              false,
		EDNSBufferSize:            1480,
		LogVerbosity:              2,
		ServeExpiredTTL:           3600,
		ServeExpiredClientTimeout: 900,
		CacheMinTTL:               60,
		CacheMaxTTL:               43200,
		Threads:                   8,
		ResourceProfile:           DefaultSettings().ResourceProfile,
		NetworkMode:               DefaultSettings().NetworkMode,
		ReverseZones:              DefaultSettings().ReverseZones,
	}
	got := result.Settings
	if got.QnameMinimisation != want.QnameMinimisation || got.Threads != want.Threads ||
		got.EDNSBufferSize != want.EDNSBufferSize || got.LogVerbosity != want.LogVerbosity ||
		got.CacheMinTTL != want.CacheMinTTL || got.CacheMaxTTL != want.CacheMaxTTL {
		t.Fatalf("unexpected candidate settings: %+v", got)
	}
	for _, directive := range []string{"qname-minimisation", "prefetch", "prefetch-key", "aggressive-nsec",
		"serve-expired", "edns-buffer-size", "verbosity", "serve-expired-ttl",
		"serve-expired-client-timeout", "cache-min-ttl", "cache-max-ttl", "num-threads"} {
		if finding := findingFor(t, result.Findings, directive); finding.Disposition != ImportGuided {
			t.Fatalf("expected %s to be guided, got %+v", directive, finding)
		}
	}
	if result.CustomAdopted != "" {
		t.Fatalf("expected no expert adoption, got %q", result.CustomAdopted)
	}
}

func TestImportRejectsMalformedGuidedValues(t *testing.T) {
	result, err := ImportUnboundConf(DefaultSettings(), "", "server:\n    num-threads: not-a-number\n")
	if err != nil {
		t.Fatal(err)
	}
	finding := findingFor(t, result.Findings, "num-threads")
	if finding.Disposition != ImportBlocked {
		t.Fatalf("expected blocked disposition for malformed value, got %+v", finding)
	}
	if result.Settings.Threads != DefaultSettings().Threads {
		t.Fatalf("malformed value must not change the candidate: %+v", result.Settings)
	}
}

func TestImportFiltersMatchingFixedBaseAndRejectsConflicts(t *testing.T) {
	result, err := ImportUnboundConf(DefaultSettings(), "", "server:\n    hide-identity: yes\n    hide-version: no\n")
	if err != nil {
		t.Fatal(err)
	}
	match := findingFor(t, result.Findings, "hide-identity")
	if match.Disposition != ImportFixedBase {
		t.Fatalf("expected matching fixed-base value to be filtered, got %+v", match)
	}
	conflict := findingFor(t, result.Findings, "hide-version")
	if conflict.Disposition != ImportBlocked || !strings.Contains(conflict.Detail, "conflicts") {
		t.Fatalf("expected a conflict rejection for hide-version, got %+v", conflict)
	}
	if result.CustomAdopted != "" {
		t.Fatalf("fixed-base directives must never be offered for expert adoption, got %q", result.CustomAdopted)
	}
}

func TestImportFiltersStructuralFixedBaseRegardlessOfValue(t *testing.T) {
	result, err := ImportUnboundConf(DefaultSettings(), "", "server:\n    root-hints: \"/some/other/path\"\n")
	if err != nil {
		t.Fatal(err)
	}
	finding := findingFor(t, result.Findings, "root-hints")
	if finding.Disposition != ImportFixedBase {
		t.Fatalf("expected root-hints to be filtered as fixed base, got %+v", finding)
	}
}

func TestImportBlocksDirectivesThatMapToUnmappedGuidedConcepts(t *testing.T) {
	for _, directive := range []string{"do-ip4", "do-ip6", "prefer-ip6", "rrset-cache-size", "msg-cache-size"} {
		result, err := ImportUnboundConf(DefaultSettings(), "", "server:\n    "+directive+": yes\n")
		if err != nil {
			t.Fatal(err)
		}
		finding := findingFor(t, result.Findings, directive)
		if finding.Disposition != ImportBlocked {
			t.Fatalf("expected %s to be blocked pending guided support, got %+v", directive, finding)
		}
		if result.CustomAdopted != "" {
			t.Fatalf("%s must not be offered for expert adoption (it would be rejected downstream anyway), got %q", directive, result.CustomAdopted)
		}
	}
}

func TestImportBlocksDirectivesFromTheExistingBlocklist(t *testing.T) {
	result, err := ImportUnboundConf(DefaultSettings(), "", "server:\n    chroot: \"\"\n    interface: 0.0.0.0\n")
	if err != nil {
		t.Fatal(err)
	}
	for _, directive := range []string{"chroot", "interface"} {
		finding := findingFor(t, result.Findings, directive)
		if finding.Disposition != ImportBlocked {
			t.Fatalf("expected %s to be blocked, got %+v", directive, finding)
		}
	}
}

func TestImportOffersUnmappedServerDirectivesForExpertAdoption(t *testing.T) {
	result, err := ImportUnboundConf(DefaultSettings(), "", "server:\n    so-reuseport: yes\n")
	if err != nil {
		t.Fatal(err)
	}
	finding := findingFor(t, result.Findings, "so-reuseport")
	if finding.Disposition != ImportExpert {
		t.Fatalf("expected so-reuseport to be offered for expert adoption, got %+v", finding)
	}
	if !strings.Contains(result.CustomAdopted, "so-reuseport: yes") {
		t.Fatalf("expected so-reuseport to be adopted into custom config, got %q", result.CustomAdopted)
	}
}

func TestImportAppliesCleanForwardZonesAsGuided(t *testing.T) {
	content := "forward-zone:\n" +
		"    name: \"corp.example.\"\n" +
		"    forward-addr: 192.0.2.53\n" +
		"    forward-addr: 192.0.2.54\n" +
		"    forward-first: yes\n" +
		"forward-zone:\n" +
		"    name: \"other.example.\"\n" +
		"    forward-addr: 192.0.2.55\n"

	result, err := ImportUnboundConf(DefaultSettings(), "", content)
	if err != nil {
		t.Fatal(err)
	}
	if count := len(result.Findings); count != 2 {
		t.Fatalf("expected one finding per forward-zone block, got %d: %+v", count, result.Findings)
	}
	for _, finding := range result.Findings {
		if finding.Section != "forward-zone" || finding.Disposition != ImportGuided {
			t.Fatalf("expected clean forward-zone blocks to be guided, got %+v", finding)
		}
	}
	if len(result.Settings.ForwardZones) != 2 {
		t.Fatalf("expected 2 guided forward zones, got %+v", result.Settings.ForwardZones)
	}
	corp := result.Settings.ForwardZones[0]
	if corp.Name != "corp.example." || len(corp.Servers) != 2 || corp.Servers[0] != "192.0.2.53" || corp.Servers[1] != "192.0.2.54" || !corp.ForwardFirst {
		t.Fatalf("unexpected first zone: %+v", corp)
	}
	other := result.Settings.ForwardZones[1]
	if other.Name != "other.example." || len(other.Servers) != 1 || other.Servers[0] != "192.0.2.55" || other.ForwardFirst {
		t.Fatalf("unexpected second zone: %+v", other)
	}
	if result.CustomAdopted != "" {
		t.Fatalf("clean forward zones must not also be offered for expert adoption, got %q", result.CustomAdopted)
	}
}

func TestImportOffersUncleanForwardZoneBlocksForExpertAdoption(t *testing.T) {
	// Missing forward-addr entirely, and a directive this importer doesn't
	// reverse-map (forward-tls-upstream) - neither is a clean fit for the
	// guided ForwardZone model, so both fall back to whole-block adoption.
	content := "forward-zone:\n" +
		"    name: \"no-target.example.\"\n" +
		"forward-zone:\n" +
		"    name: \"tls.example.\"\n" +
		"    forward-addr: 192.0.2.53\n" +
		"    forward-tls-upstream: yes\n"

	result, err := ImportUnboundConf(DefaultSettings(), "", content)
	if err != nil {
		t.Fatal(err)
	}
	if count := len(result.Findings); count != 2 {
		t.Fatalf("expected one finding per forward-zone block, got %d: %+v", count, result.Findings)
	}
	for _, finding := range result.Findings {
		if finding.Section != "forward-zone" || finding.Disposition != ImportExpert {
			t.Fatalf("unexpected finding: %+v", finding)
		}
	}
	if len(result.Settings.ForwardZones) != 0 {
		t.Fatalf("expected no guided zones from unclean blocks, got %+v", result.Settings.ForwardZones)
	}
	if strings.Count(result.CustomAdopted, "forward-zone:") != 2 {
		t.Fatalf("expected both forward-zone blocks preserved, got %q", result.CustomAdopted)
	}
	if !strings.Contains(result.CustomAdopted, `name: "no-target.example."`) || !strings.Contains(result.CustomAdopted, `name: "tls.example."`) {
		t.Fatalf("expected both zone names preserved, got %q", result.CustomAdopted)
	}
}

func TestImportBlocksDangerousSections(t *testing.T) {
	content := "remote-control:\n    control-enable: yes\n"
	result, err := ImportUnboundConf(DefaultSettings(), "", content)
	if err != nil {
		t.Fatal(err)
	}
	finding := findingFor(t, result.Findings, "remote-control")
	if finding.Disposition != ImportBlocked {
		t.Fatalf("expected remote-control section to be blocked, got %+v", finding)
	}
	if result.CustomAdopted != "" {
		t.Fatalf("a blocked section must never be adopted, got %q", result.CustomAdopted)
	}
}

func TestImportAppendsToExistingCustomConfig(t *testing.T) {
	existing := "server:\n    so-reuseport: yes\n"
	result, err := ImportUnboundConf(DefaultSettings(), existing, "server:\n    so-rcvbuf: 4m\n")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(result.CustomAdopted, existing) {
		t.Fatalf("expected existing custom config preserved at the start, got %q", result.CustomAdopted)
	}
	if !strings.Contains(result.CustomAdopted, "so-rcvbuf: 4m") {
		t.Fatalf("expected the newly adopted directive appended, got %q", result.CustomAdopted)
	}
}

func TestImportIgnoresCommentsAndBlankLines(t *testing.T) {
	content := "# a comment\n\nserver:\n    # another comment\n    num-threads: 4 # inline comment\n\n"
	result, err := ImportUnboundConf(DefaultSettings(), "", content)
	if err != nil {
		t.Fatal(err)
	}
	if result.Settings.Threads != 4 {
		t.Fatalf("expected num-threads to be parsed despite comments, got %+v", result.Settings)
	}
}

func TestImportRejectsOversizedInput(t *testing.T) {
	oversized := strings.Repeat("a", MaxCustomConfigBytes+1)
	if _, err := ImportUnboundConf(DefaultSettings(), "", oversized); err == nil {
		t.Fatal("expected an error for oversized input")
	}
}

// Regression test: domain-insecure/private-domain lines are classified after
// forward zones are extracted, and must still be reflected in the zones
// actually returned in Settings - an earlier version copied the extracted
// zones into candidate.ForwardZones before these opt-ins were applied, so
// the flags were silently lost.
func TestImportAppliesZoneScopedOptInsToTheReturnedZone(t *testing.T) {
	content := "server:\n" +
		"    domain-insecure: \"corp.example.\"\n" +
		"    private-domain: \"corp.example.\"\n" +
		"forward-zone:\n" +
		"    name: \"corp.example.\"\n" +
		"    forward-addr: 192.0.2.53\n"

	result, err := ImportUnboundConf(DefaultSettings(), "", content)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Settings.ForwardZones) != 1 {
		t.Fatalf("expected 1 guided forward zone, got %+v", result.Settings.ForwardZones)
	}
	zone := result.Settings.ForwardZones[0]
	if !zone.AllowUnsigned || !zone.AllowPrivateAddresses {
		t.Fatalf("expected both zone-scoped opt-ins applied, got %+v", zone)
	}
	for _, directive := range []string{"domain-insecure", "private-domain"} {
		if finding := findingFor(t, result.Findings, directive); finding.Disposition != ImportGuided {
			t.Fatalf("expected %s to be guided, got %+v", directive, finding)
		}
	}
	if len(result.Settings.PrivateDomains) != 0 {
		t.Fatalf("a zone-scoped private-domain must not also land in the global list, got %+v", result.Settings.PrivateDomains)
	}
}

func TestImportRoutesUnmatchedPrivateDomainToTheGlobalList(t *testing.T) {
	content := "server:\n    private-domain: \"home.example.\"\n"
	result, err := ImportUnboundConf(DefaultSettings(), "", content)
	if err != nil {
		t.Fatal(err)
	}
	finding := findingFor(t, result.Findings, "private-domain")
	if finding.Disposition != ImportGuided {
		t.Fatalf("expected private-domain to be guided, got %+v", finding)
	}
	if len(result.Settings.PrivateDomains) != 1 || result.Settings.PrivateDomains[0] != "home.example." {
		t.Fatalf("expected home.example. in the global private domain list, got %+v", result.Settings.PrivateDomains)
	}
}

func TestImportOffersUnmatchedDomainInsecureForExpertAdoption(t *testing.T) {
	content := "server:\n    domain-insecure: \"no-such-zone.example.\"\n"
	result, err := ImportUnboundConf(DefaultSettings(), "", content)
	if err != nil {
		t.Fatal(err)
	}
	finding := findingFor(t, result.Findings, "domain-insecure")
	if finding.Disposition != ImportExpert {
		t.Fatalf("expected an orphaned domain-insecure to be offered for expert adoption, got %+v", finding)
	}
	if !strings.Contains(result.CustomAdopted, `domain-insecure: "no-such-zone.example."`) {
		t.Fatalf("expected domain-insecure adopted verbatim, got %q", result.CustomAdopted)
	}
}

func TestImportAppliesCleanLocalZoneAsGuided(t *testing.T) {
	content := "server:\n" +
		"    local-zone: \"home.lab.\" static\n" +
		"    local-data: \"printer.home.lab. IN A 192.168.1.20\"\n" +
		"    local-data: \"printer.home.lab. IN AAAA fd00::20\"\n" +
		"    local-data-ptr: \"192.168.1.20 printer.home.lab\"\n" +
		"    local-data: \"router.home.lab. IN A 192.168.1.1\"\n"

	result, err := ImportUnboundConf(DefaultSettings(), "", content)
	if err != nil {
		t.Fatal(err)
	}
	finding := findingFor(t, result.Findings, "local-zone")
	if finding.Disposition != ImportGuided || finding.Value != "home.lab." {
		t.Fatalf("expected local-zone to be guided, got %+v", finding)
	}
	if len(result.Settings.LocalZones) != 1 {
		t.Fatalf("expected 1 guided local zone, got %+v", result.Settings.LocalZones)
	}
	zone := result.Settings.LocalZones[0]
	if zone.Name != "home.lab." || len(zone.Hosts) != 2 {
		t.Fatalf("unexpected zone: %+v", zone)
	}
	printer := zone.Hosts[0]
	if printer.Hostname != "printer" || printer.IPv4 != "192.168.1.20" || printer.IPv6 != "fd00::20" || !printer.PTR {
		t.Fatalf("unexpected first host: %+v", printer)
	}
	router := zone.Hosts[1]
	if router.Hostname != "router" || router.IPv4 != "192.168.1.1" || router.PTR {
		t.Fatalf("unexpected second host: %+v", router)
	}
	if result.CustomAdopted != "" {
		t.Fatalf("a clean local zone must not also be offered for expert adoption, got %q", result.CustomAdopted)
	}
}

func TestImportHandlesMultipleLocalZonesInOrder(t *testing.T) {
	content := "server:\n" +
		"    local-zone: \"home.lab.\" static\n" +
		"    local-data: \"printer.home.lab. IN A 192.168.1.20\"\n" +
		"    local-zone: \"guests.lab.\" static\n" +
		"    local-data: \"tablet.guests.lab. IN A 192.168.2.20\"\n"

	result, err := ImportUnboundConf(DefaultSettings(), "", content)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Settings.LocalZones) != 2 {
		t.Fatalf("expected 2 guided local zones, got %+v", result.Settings.LocalZones)
	}
	if result.Settings.LocalZones[0].Name != "home.lab." || result.Settings.LocalZones[1].Name != "guests.lab." {
		t.Fatalf("unexpected zone order: %+v", result.Settings.LocalZones)
	}
}

func TestImportNeverTreatsReservedRFC1918NamesAsHostInventory(t *testing.T) {
	// Only one of 172.16.0.0/12's 16 required reverse zones is present, so
	// this is neither a complete reverse-zone policy nor a valid host
	// inventory zone (it has no local-data at all) - it must fall back to
	// expert adoption, not silently become an empty LocalZone.
	content := "server:\n    local-zone: \"16.172.in-addr.arpa.\" static\n"
	result, err := ImportUnboundConf(DefaultSettings(), "", content)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Settings.LocalZones) != 0 {
		t.Fatalf("expected RFC1918 reverse zone not to be treated as a host inventory zone, got %+v", result.Settings.LocalZones)
	}
	finding := findingFor(t, result.Findings, "local-zone")
	if finding.Disposition != ImportExpert {
		t.Fatalf("expected the incomplete reverse-zone set to fall back to expert adoption, got %+v", finding)
	}
}

func TestImportFallsBackToExpertForCNAMERecords(t *testing.T) {
	content := "server:\n" +
		"    local-zone: \"home.lab.\" static\n" +
		"    local-data: \"alias.home.lab. IN CNAME target.example.\"\n"

	result, err := ImportUnboundConf(DefaultSettings(), "", content)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Settings.LocalZones) != 0 {
		t.Fatalf("expected CNAME group not to be guided-mapped, got %+v", result.Settings.LocalZones)
	}
	if !strings.Contains(result.CustomAdopted, `local-zone: "home.lab." static`) || !strings.Contains(result.CustomAdopted, "CNAME") {
		t.Fatalf("expected the whole group preserved for expert adoption, got %q", result.CustomAdopted)
	}
}

func TestImportFallsBackToExpertForMismatchedPTR(t *testing.T) {
	content := "server:\n" +
		"    local-zone: \"home.lab.\" static\n" +
		"    local-data: \"printer.home.lab. IN A 192.168.1.20\"\n" +
		"    local-data-ptr: \"192.168.1.99 printer.home.lab\"\n"

	result, err := ImportUnboundConf(DefaultSettings(), "", content)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Settings.LocalZones) != 0 {
		t.Fatalf("expected a mismatched PTR to prevent guided mapping, got %+v", result.Settings.LocalZones)
	}
	if !strings.Contains(result.CustomAdopted, "local-data-ptr") {
		t.Fatalf("expected the whole group preserved for expert adoption, got %q", result.CustomAdopted)
	}
}

func TestImportFallsBackToExpertForNonStaticLocalZoneType(t *testing.T) {
	content := "server:\n    local-zone: \"custom.example.\" transparent\n"
	result, err := ImportUnboundConf(DefaultSettings(), "", content)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Settings.LocalZones) != 0 {
		t.Fatalf("expected a non-static local-zone type not to be guided-mapped, got %+v", result.Settings.LocalZones)
	}
	finding := findingFor(t, result.Findings, "local-zone")
	if finding.Disposition != ImportExpert {
		t.Fatalf("expected expert adoption, got %+v", finding)
	}
}

func TestImportAppliesReverseZonePolicyForSingleZoneNetwork(t *testing.T) {
	content := "server:\n    local-zone: \"10.in-addr.arpa.\" transparent\n"
	result, err := ImportUnboundConf(DefaultSettings(), "", content)
	if err != nil {
		t.Fatal(err)
	}
	finding := findingFor(t, result.Findings, "local-zone")
	if finding.Disposition != ImportGuided || finding.Value != "10.0.0.0/8" {
		t.Fatalf("expected the reverse-zone policy to be guided, got %+v", finding)
	}
	var got10, got172, got192 string
	for _, policy := range result.Settings.ReverseZones {
		switch policy.Network {
		case "10.0.0.0/8":
			got10 = policy.Mode
		case "172.16.0.0/12":
			got172 = policy.Mode
		case "192.168.0.0/16":
			got192 = policy.Mode
		}
	}
	if got10 != reverseModeTransparent {
		t.Fatalf("expected 10.0.0.0/8 to become transparent, got %q", got10)
	}
	if got172 != reverseModeNXDOMAIN || got192 != reverseModeNXDOMAIN {
		t.Fatalf("expected the other networks to keep their default, got 172=%q 192=%q", got172, got192)
	}
	if result.CustomAdopted != "" {
		t.Fatalf("a clean reverse-zone policy must not also be offered for expert adoption, got %q", result.CustomAdopted)
	}
}

func TestImportAppliesReverseZonePolicyForMultiZoneNetwork(t *testing.T) {
	var content strings.Builder
	content.WriteString("server:\n")
	for _, zone := range reverse172Zones() {
		content.WriteString("    local-zone: \"" + zone + "\" static\n")
	}
	result, err := ImportUnboundConf(DefaultSettings(), "", content.String())
	if err != nil {
		t.Fatal(err)
	}
	if count := len(result.Findings); count != 16 {
		t.Fatalf("expected one finding per reverse zone line, got %d", count)
	}
	for _, policy := range result.Settings.ReverseZones {
		if policy.Network == "172.16.0.0/12" && policy.Mode != reverseModeNXDOMAIN {
			t.Fatalf("expected 172.16.0.0/12 to become nxdomain, got %q", policy.Mode)
		}
	}
}

func TestImportRejectsInconsistentReverseZoneTypes(t *testing.T) {
	content := "server:\n" +
		"    local-zone: \"10.in-addr.arpa.\" static\n"
	// Only a single-zone network can even be "inconsistent" by using an
	// unrecognized type keyword - simulate that instead of a real type
	// mismatch, which needs 2+ zones per network.
	content = "server:\n    local-zone: \"10.in-addr.arpa.\" refuse\n"
	result, err := ImportUnboundConf(DefaultSettings(), "", content)
	if err != nil {
		t.Fatal(err)
	}
	for _, policy := range result.Settings.ReverseZones {
		if policy.Network == "10.0.0.0/8" && policy.Mode != reverseModeNXDOMAIN {
			t.Fatalf("expected the unrecognized type to leave the default policy untouched, got %q", policy.Mode)
		}
	}
	finding := findingFor(t, result.Findings, "local-zone")
	if finding.Disposition != ImportExpert {
		t.Fatalf("expected an unrecognized local-zone type to fall back to expert adoption, got %+v", finding)
	}
}

func TestImportAppliesNetworkModeWhenTheTripleIsConsistent(t *testing.T) {
	content := "server:\n    do-ip4: no\n    do-ip6: yes\n    prefer-ip6: yes\n"
	result, err := ImportUnboundConf(DefaultSettings(), "", content)
	if err != nil {
		t.Fatal(err)
	}
	if result.Settings.NetworkMode != networkModeIPv6 {
		t.Fatalf("expected ipv6 network mode, got %q", result.Settings.NetworkMode)
	}
	for _, directive := range []string{"do-ip4", "do-ip6", "prefer-ip6"} {
		if finding := findingFor(t, result.Findings, directive); finding.Disposition != ImportGuided {
			t.Fatalf("expected %s to be guided, got %+v", directive, finding)
		}
	}
}

func TestImportLeavesNetworkModeUnmappedWhenIncomplete(t *testing.T) {
	content := "server:\n    do-ip4: no\n"
	result, err := ImportUnboundConf(DefaultSettings(), "", content)
	if err != nil {
		t.Fatal(err)
	}
	if result.Settings.NetworkMode != DefaultSettings().NetworkMode {
		t.Fatalf("expected network mode to stay at the default, got %q", result.Settings.NetworkMode)
	}
	finding := findingFor(t, result.Findings, "do-ip4")
	if finding.Disposition != ImportBlocked {
		t.Fatalf("expected an incomplete triple to be blocked, got %+v", finding)
	}
}

func TestImportAppliesResourceProfileWhenSizesMatchAPreset(t *testing.T) {
	content := "server:\n    rrset-cache-size: 128m\n    msg-cache-size: 64m\n"
	result, err := ImportUnboundConf(DefaultSettings(), "", content)
	if err != nil {
		t.Fatal(err)
	}
	if result.Settings.ResourceProfile != resourceProfileLarge {
		t.Fatalf("expected the large resource profile, got %q", result.Settings.ResourceProfile)
	}
	for _, directive := range []string{"rrset-cache-size", "msg-cache-size"} {
		if finding := findingFor(t, result.Findings, directive); finding.Disposition != ImportGuided {
			t.Fatalf("expected %s to be guided, got %+v", directive, finding)
		}
	}
}

func TestImportLeavesResourceProfileUnmappedWhenSizesDontMatch(t *testing.T) {
	content := "server:\n    rrset-cache-size: 999m\n    msg-cache-size: 64m\n"
	result, err := ImportUnboundConf(DefaultSettings(), "", content)
	if err != nil {
		t.Fatal(err)
	}
	if result.Settings.ResourceProfile != DefaultSettings().ResourceProfile {
		t.Fatalf("expected the resource profile to stay at the default, got %q", result.Settings.ResourceProfile)
	}
	finding := findingFor(t, result.Findings, "rrset-cache-size")
	if finding.Disposition != ImportBlocked {
		t.Fatalf("expected an unmatched size pair to be blocked, got %+v", finding)
	}
}

// Regression test: a forward-zone block whose name isn't a canonical FQDN
// (missing the trailing dot here) used to be accepted as a clean guided
// match, only to fail much later with a raw Settings.Validate() error at
// the "Validate with Unbound" step - confusing, since the classifier had
// already called it "guided". It must fall back to expert adoption instead.
func TestImportRejectsForwardZoneWithNonCanonicalName(t *testing.T) {
	content := "forward-zone:\n" +
		"    name: \"corp.example\"\n" +
		"    forward-addr: 192.0.2.53\n"
	result, err := ImportUnboundConf(DefaultSettings(), "", content)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Settings.ForwardZones) != 0 {
		t.Fatalf("expected a non-canonical zone name not to be guided-mapped, got %+v", result.Settings.ForwardZones)
	}
	finding := findingFor(t, result.Findings, "forward-zone")
	if finding.Disposition != ImportExpert {
		t.Fatalf("expected expert adoption, got %+v", finding)
	}
	if !strings.Contains(result.CustomAdopted, `name: "corp.example"`) {
		t.Fatalf("expected the zone preserved verbatim for expert adoption, got %q", result.CustomAdopted)
	}
}

func TestImportRejectsForwardZoneWithMalformedTarget(t *testing.T) {
	content := "forward-zone:\n" +
		"    name: \"corp.example.\"\n" +
		"    forward-addr: 192.0.2.53@853\n"
	result, err := ImportUnboundConf(DefaultSettings(), "", content)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Settings.ForwardZones) != 0 {
		t.Fatalf("expected a port-suffixed target not to be guided-mapped, got %+v", result.Settings.ForwardZones)
	}
	finding := findingFor(t, result.Findings, "forward-zone")
	if finding.Disposition != ImportExpert {
		t.Fatalf("expected expert adoption, got %+v", finding)
	}
}

func TestImportRejectsForwardZoneTargetingRootGuardInternals(t *testing.T) {
	content := "forward-zone:\n" +
		"    name: \"corp.example.\"\n" +
		"    forward-addr: 127.0.0.1\n"
	result, err := ImportUnboundConf(DefaultSettings(), "", content)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Settings.ForwardZones) != 0 {
		t.Fatalf("expected a loopback target not to be guided-mapped, got %+v", result.Settings.ForwardZones)
	}
}

func TestImportRejectsLocalZoneWithNonCanonicalName(t *testing.T) {
	content := "server:\n" +
		"    local-zone: \"Home.Lab.\" static\n" +
		"    local-data: \"printer.Home.Lab. IN A 192.168.1.20\"\n"
	result, err := ImportUnboundConf(DefaultSettings(), "", content)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Settings.LocalZones) != 0 {
		t.Fatalf("expected a non-canonical (mixed-case) zone name not to be guided-mapped, got %+v", result.Settings.LocalZones)
	}
	finding := findingFor(t, result.Findings, "local-zone")
	if finding.Disposition != ImportExpert {
		t.Fatalf("expected expert adoption, got %+v", finding)
	}
}

func TestImportRejectsLocalZoneWithMalformedHostAddress(t *testing.T) {
	content := "server:\n" +
		"    local-zone: \"home.lab.\" static\n" +
		"    local-data: \"printer.home.lab. IN A 300.168.1.20\"\n"
	result, err := ImportUnboundConf(DefaultSettings(), "", content)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Settings.LocalZones) != 0 {
		t.Fatalf("expected an invalid IPv4 address not to be guided-mapped, got %+v", result.Settings.LocalZones)
	}
}

// Regression test: a hand-written unbound.conf is free to keep the trailing
// dot on a local-data-ptr target ("host.zone." rather than Render()'s own
// "host.zone") - both are valid, equivalent DNS names. Reported live: a
// real 11-host zone with trailing-dot PTR targets fell back to expert
// adoption entirely, even though every record was perfectly clean data.
func TestImportAcceptsPTRTargetWithTrailingDot(t *testing.T) {
	content := "server:\n" +
		"    local-zone: \"home.lab.\" static\n" +
		"    local-data: \"printer.home.lab. IN A 192.168.1.20\"\n" +
		"    local-data-ptr: \"192.168.1.20 printer.home.lab.\"\n"
	result, err := ImportUnboundConf(DefaultSettings(), "", content)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Settings.LocalZones) != 1 {
		t.Fatalf("expected the zone to be guided-mapped despite the trailing dot, got %+v", result.Settings.LocalZones)
	}
	if host := result.Settings.LocalZones[0].Hosts[0]; !host.PTR {
		t.Fatalf("expected PTR to be recognized, got %+v", host)
	}
}

// Regression test: re-importing a file after its zones were already
// activated used to blindly append a second copy, producing a "duplicates"
// validation error on an otherwise unchanged re-import. A zone name that
// already exists is now replaced in place instead - re-importing the same
// file is idempotent, and re-importing an edited file updates the zone.
func TestImportReplacesExistingZonesByNameInsteadOfDuplicating(t *testing.T) {
	current := DefaultSettings()
	current.LocalZones = []LocalZone{{
		Name:  "home.lab.",
		Hosts: []LocalHost{{Hostname: "printer", IPv4: "192.168.1.20", PTR: true}},
	}}
	current.ForwardZones = []ForwardZone{{Name: "corp.example.", Servers: []string{"192.0.2.53"}}}

	content := "server:\n" +
		"    local-zone: \"home.lab.\" static\n" +
		"    local-data: \"router.home.lab. IN A 192.168.1.1\"\n" +
		"forward-zone:\n" +
		"    name: \"corp.example.\"\n" +
		"    forward-addr: 192.0.2.54\n"

	result, err := ImportUnboundConf(current, "", content)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Settings.LocalZones) != 1 {
		t.Fatalf("expected the existing zone to be replaced, not duplicated, got %+v", result.Settings.LocalZones)
	}
	if hosts := result.Settings.LocalZones[0].Hosts; len(hosts) != 1 || hosts[0].Hostname != "router" {
		t.Fatalf("expected the zone updated to the freshly imported content, got %+v", hosts)
	}
	if len(result.Settings.ForwardZones) != 1 || result.Settings.ForwardZones[0].Servers[0] != "192.0.2.54" {
		t.Fatalf("expected the existing forward zone replaced, not duplicated, got %+v", result.Settings.ForwardZones)
	}
}

// A genuinely repeated re-import (the exact same file, unchanged) must be a
// true no-op: identical content in, identical settings out.
func TestImportOfTheSameFileTwiceIsIdempotent(t *testing.T) {
	content := "server:\n" +
		"    local-zone: \"home.lab.\" static\n" +
		"    local-data: \"printer.home.lab. IN A 192.168.1.20\"\n"

	first, err := ImportUnboundConf(DefaultSettings(), "", content)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ImportUnboundConf(first.Settings, "", content)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Settings.LocalZones) != 1 {
		t.Fatalf("expected re-importing the identical file to stay at 1 zone, got %+v", second.Settings.LocalZones)
	}
}
