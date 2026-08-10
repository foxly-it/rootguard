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
