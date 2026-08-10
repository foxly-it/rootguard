package unbound

import (
	"fmt"
	"net/netip"
	"strconv"
	"strings"
)

// ImportDisposition classifies one parsed directive from an imported
// unbound.conf against the ownership model in
// docs/unbound-configuration-roadmap.md.
type ImportDisposition string

const (
	// ImportGuided means the directive was applied to the candidate Settings.
	ImportGuided ImportDisposition = "guided"
	// ImportFixedBase means RootGuard already sets this; filtered out unless
	// the imported value conflicts with what RootGuard enforces.
	ImportFixedBase ImportDisposition = "fixed_base"
	// ImportExpert means the directive has no guided equivalent (yet) but is
	// allowed in the expert custom config, so it's offered for adoption there.
	ImportExpert ImportDisposition = "expert"
	// ImportBlocked means the directive cannot be adopted from the browser at
	// all, or maps to a guided concept this importer doesn't reverse-map yet.
	ImportBlocked ImportDisposition = "blocked"
)

type ImportFinding struct {
	Section     string            `json:"section"`
	Line        int               `json:"line"`
	Directive   string            `json:"directive"`
	Value       string            `json:"value,omitempty"`
	Disposition ImportDisposition `json:"disposition"`
	Detail      string            `json:"detail"`
}

// ImportResult is the outcome of classifying a pasted/uploaded unbound.conf:
// Settings is the candidate after applying every guided finding on top of
// the settings passed in, and CustomAdopted is the current custom config
// with every expert-adoptable finding appended. Both are previews - nothing
// is written until the caller runs them through the normal preview/activate
// lifecycle (PreviewBundle/ApplyBundle), same as any other import.
type ImportResult struct {
	Findings      []ImportFinding `json:"findings"`
	Settings      Settings        `json:"settings"`
	CustomAdopted string          `json:"custom_adopted"`
}

type parsedDirective struct {
	key   string
	value string
	line  int
	raw   string
}

type parsedBlock struct {
	section    string
	lineStart  int
	directives []parsedDirective
}

// blockedImportSections are Unbound clauses that must never be adopted into
// the expert custom config, regardless of their content: they open a control
// surface (remote-control, dynamically loaded code) or a persistent
// query-log sink (dnstap), each already disallowed for manual expert input
// for the same reason (see blockedDirectives).
var blockedImportSections = map[string]string{
	"remote-control": "remote control is not exposed by RootGuard",
	"python":         "dynamically loaded modules are not allowed",
	"dynlib":         "dynamically loaded modules are not allowed",
	"dnstap":         "persistent per-query logging is not allowed; use temporary diagnostics instead",
}

// fixedBaseScalars are simple directives RootGuard's own image already sets
// (rootguard-unbound/unbound.conf) that are safe to exact-match compare - an
// imported value that differs is a real conflict, not just redundant
// restatement of what's already active.
var fixedBaseScalars = map[string]string{
	"hide-identity":            "yes",
	"hide-version":             "yes",
	"harden-glue":              "yes",
	"harden-dnssec-stripped":   "yes",
	"harden-below-nxdomain":    "yes",
	"unwanted-reply-threshold": "10000000",
}

// fixedBaseStructural are fixed-base directives whose value is a container
// file path or similar - always filtered without a value comparison, since
// an imported path is never meaningfully comparable to RootGuard's own.
var fixedBaseStructural = map[string]struct{}{
	"root-hints":             {},
	"auto-trust-anchor-file": {},
	"trust-anchor-file":      {},
}

// guidedNotYetMapped is the fallback for directives that DO belong to a
// guided setting but that extractNetworkMode/extractResourceProfile above
// couldn't cleanly resolve (a missing/duplicated directive in the trio or
// pair, or a combination/cache-size pair that doesn't match a known
// RootGuard preset). Classified as blocked rather than expert so they don't
// silently duplicate a guided concept in the custom config, where they'd be
// rejected anyway by blockedDirectives - the point is an accurate up-front
// finding, not a deferred failure.
var guidedNotYetMapped = map[string]string{
	"do-ip4":           "part of the guided network mode, but do-ip4/do-ip6/prefer-ip6 must all be present with a consistent combination for this importer to apply it - set it directly in the guided form",
	"do-ip6":           "part of the guided network mode, but do-ip4/do-ip6/prefer-ip6 must all be present with a consistent combination for this importer to apply it - set it directly in the guided form",
	"prefer-ip6":       "part of the guided network mode, but do-ip4/do-ip6/prefer-ip6 must all be present with a consistent combination for this importer to apply it - set it directly in the guided form",
	"rrset-cache-size": "part of the guided resource profile, but the cache sizes don't match a known RootGuard profile (small/medium/large) - set it directly in the guided form",
	"msg-cache-size":   "part of the guided resource profile, but the cache sizes don't match a known RootGuard profile (small/medium/large) - set it directly in the guided form",
}

// ImportUnboundConf parses content as an unbound.conf and classifies every
// directive against current/currentCustom. It never mutates its inputs.
func ImportUnboundConf(current Settings, currentCustom string, content string) (ImportResult, error) {
	if len(content) > MaxCustomConfigBytes {
		return ImportResult{}, fmt.Errorf("%w: maximum size is %d bytes", ErrInvalidCustomConfig, MaxCustomConfigBytes)
	}

	blocks := parseUnboundConf(content)
	candidate := current
	var findings []ImportFinding
	var expertLines []string

	// Forward zones are extracted first: domain-insecure/private-domain
	// lines in the server: block only make sense once the set of zone names
	// they might refer to is already known (see classifyServerDirective's
	// callers below).
	zones, zoneNames, mappedBlocks, zoneFindings := extractForwardZones(blocks)
	findings = append(findings, zoneFindings...)

	// Local host-inventory zones (local-zone "static" + their local-data/
	// local-data-ptr lines) are extracted the same way, independently of
	// forward zones - consumedDirectives marks exactly which directive
	// indexes within each server: block were absorbed into a guided
	// LocalZone, so the main loop below can skip just those lines.
	localZones, consumedDirectives, localZoneFindings := extractLocalZones(blocks)
	findings = append(findings, localZoneFindings...)

	// RFC1918 reverse-zone policy, network mode, and resource profile are
	// each either a fixed set of local-zone lines or a small directive
	// combination - all-or-nothing the same way network mode's three
	// directives are: partial or inconsistent input isn't guessed at, it
	// falls through to guidedNotYetMapped's blocked fallback below.
	reversePolicies, reverseConsumed, reverseFindings := extractReverseZonePolicies(blocks)
	findings = append(findings, reverseFindings...)
	mergeConsumedDirectives(consumedDirectives, reverseConsumed)

	networkMode, networkModeConsumed, networkModeFindings := extractNetworkMode(blocks)
	findings = append(findings, networkModeFindings...)
	mergeConsumedDirectives(consumedDirectives, networkModeConsumed)

	resourceProfile, resourceProfileConsumed, resourceProfileFindings := extractResourceProfile(blocks)
	findings = append(findings, resourceProfileFindings...)
	mergeConsumedDirectives(consumedDirectives, resourceProfileConsumed)

	var privateDomains []string
	for blockIndex, block := range blocks {
		if mappedBlocks[blockIndex] {
			continue
		}
		if reason, blocked := blockedImportSections[block.section]; blocked {
			findings = append(findings, ImportFinding{
				Section: block.section, Line: block.lineStart, Directive: block.section,
				Disposition: ImportBlocked, Detail: reason,
			})
			continue
		}
		if block.section != "server" {
			// No guided mapping yet for this clause (stub-zone, view, ... -
			// or a forward-zone that didn't map cleanly) - offer it for
			// expert adoption as a whole, exactly as if it had been pasted
			// into the expert editor by hand.
			findings = append(findings, ImportFinding{
				Section: block.section, Line: block.lineStart, Directive: block.section,
				Disposition: ImportExpert,
				Detail:      "no guided mapping yet for this clause - offered for expert adoption",
			})
			expertLines = append(expertLines, block.section+":")
			for _, d := range block.directives {
				expertLines = append(expertLines, "    "+d.raw)
			}
			continue
		}
		for directiveIndex, d := range block.directives {
			if consumedDirectives[blockIndex][directiveIndex] {
				continue
			}
			if d.key == "domain-insecure" || d.key == "private-domain" {
				finding, appendedDomain := classifyZoneScopedDirective(zones, zoneNames, d)
				findings = append(findings, finding)
				if appendedDomain != "" {
					privateDomains = append(privateDomains, appendedDomain)
				}
				if finding.Disposition == ImportExpert {
					expertLines = append(expertLines, d.raw)
				}
				continue
			}
			finding := classifyServerDirective(&candidate, d)
			findings = append(findings, finding)
			if finding.Disposition == ImportExpert {
				expertLines = append(expertLines, d.raw)
			}
		}
	}
	if len(zones) > 0 {
		candidate.ForwardZones = append(append([]ForwardZone{}, candidate.ForwardZones...), zones...)
	}
	if len(localZones) > 0 {
		candidate.LocalZones = append(append([]LocalZone{}, candidate.LocalZones...), localZones...)
	}
	if len(privateDomains) > 0 {
		candidate.PrivateDomains = append(append([]string{}, candidate.PrivateDomains...), privateDomains...)
	}
	for i := range candidate.ReverseZones {
		if mode, ok := reversePolicies[candidate.ReverseZones[i].Network]; ok {
			candidate.ReverseZones[i].Mode = mode
		}
	}
	if networkMode != "" {
		candidate.NetworkMode = networkMode
	}
	if resourceProfile != "" {
		candidate.ResourceProfile = resourceProfile
	}

	adopted := strings.TrimSpace(currentCustom)
	if len(expertLines) > 0 {
		if adopted != "" {
			adopted += "\n\n"
		}
		adopted += strings.Join(expertLines, "\n") + "\n"
	}
	return ImportResult{Findings: findings, Settings: candidate, CustomAdopted: adopted}, nil
}

// extractForwardZones reverse-maps forward-zone blocks that map cleanly onto
// the guided ForwardZone model: a name, at least one forward-addr, and
// nothing else RootGuard doesn't already render (forward-first is fine;
// anything else - forward-tls-upstream, a port-suffixed forward-addr, and so
// on - means the block isn't a clean fit, so it's left unmapped and falls
// through to whole-block expert adoption instead of silently dropping the
// directives that don't fit). mappedBlocks marks which block indexes were
// consumed here so the caller's main loop skips them.
func extractForwardZones(blocks []parsedBlock) (zones []ForwardZone, zoneNames map[string]struct{}, mappedBlocks map[int]bool, findings []ImportFinding) {
	zoneNames = map[string]struct{}{}
	mappedBlocks = map[int]bool{}
	for blockIndex, block := range blocks {
		if block.section != "forward-zone" {
			continue
		}
		zone := ForwardZone{}
		clean := true
		for _, d := range block.directives {
			switch d.key {
			case "name":
				zone.Name = d.value
			case "forward-addr":
				zone.Servers = append(zone.Servers, d.value)
			case "forward-first":
				zone.ForwardFirst = strings.EqualFold(d.value, "yes")
			default:
				clean = false
			}
		}
		if !clean || zone.Name == "" || len(zone.Servers) == 0 || validateCanonicalZoneName(zone.Name) != nil {
			// Not left in mappedBlocks, so the caller's main loop falls back
			// to its generic whole-block expert-adoption handling for this
			// block - do not also emit a finding for it here.
			continue
		}
		targetsClean := true
		for _, server := range zone.Servers {
			if !validForwardTarget(server) {
				targetsClean = false
				break
			}
		}
		if !targetsClean {
			continue
		}
		zones = append(zones, zone)
		zoneNames[zone.Name] = struct{}{}
		mappedBlocks[blockIndex] = true
		findings = append(findings, ImportFinding{
			Section: "forward-zone", Line: block.lineStart, Directive: "forward-zone", Value: zone.Name,
			Disposition: ImportGuided, Detail: "applied as a guided conditional-forwarding zone",
		})
	}
	return zones, zoneNames, mappedBlocks, findings
}

// validForwardTarget mirrors the forward_zones[].servers[] checks in
// Settings.Validate() (canonical address, not unspecified/loopback/
// multicast/link-local/RootGuard-internal) - checked up front so a
// port-suffixed or otherwise malformed forward-addr value falls back to
// whole-block expert adoption instead of a confusing deferred validation
// error after the classifier already called it "guided".
func validForwardTarget(value string) bool {
	address, err := netip.ParseAddr(value)
	if err != nil || address.String() != value {
		return false
	}
	routedAddress := address.Unmap()
	if routedAddress.IsUnspecified() || routedAddress.IsLoopback() || routedAddress.IsMulticast() || routedAddress.IsLinkLocalUnicast() || rootGuardDNSNetwork.Contains(routedAddress) {
		return false
	}
	return true
}

// reservedReverseZoneNames returns every RFC1918 reverse-zone name RootGuard
// itself can already generate (see rfc1918ReverseZones) - these must never be
// reverse-mapped as local host-inventory zones, since they're RootGuard's own
// reverse-DNS policy zones (a different guided concept this importer doesn't
// reverse-map yet) and typically carry no host data at all, which would
// otherwise produce an empty, invalid LocalZone.
func reservedReverseZoneNames() map[string]struct{} {
	reserved := map[string]struct{}{}
	for _, zones := range rfc1918ReverseZones {
		for _, zone := range zones {
			reserved[zone] = struct{}{}
		}
	}
	return reserved
}

// extractLocalZones reverse-maps local-zone "static" clauses (plus their
// local-data/local-data-ptr lines) onto the guided host-inventory model. A
// group is a local-zone line followed immediately by zero or more
// local-data/local-data-ptr lines - exactly the contiguous shape
// Settings.Render() itself produces, which keeps export/import round-trips
// lossless. Any other server: directive interleaved between groups simply
// closes the current group without disturbing it.
//
// A group only maps cleanly if: the zone isn't one of RootGuard's own
// RFC1918 reverse zones, every local-data line is an A/AAAA record whose
// name is <hostname>.<zone> (CNAME and any other record type has no guided
// equivalent - see issue #131), and every local-data-ptr line's target and
// address match a host already established by a local-data line in the same
// group. A group that doesn't map cleanly is left entirely unconsumed, so
// the caller's main loop offers each of its lines for expert adoption
// individually instead of silently dropping anything.
func extractLocalZones(blocks []parsedBlock) (zones []LocalZone, consumed map[int]map[int]bool, findings []ImportFinding) {
	consumed = map[int]map[int]bool{}
	reserved := reservedReverseZoneNames()
	for blockIndex, block := range blocks {
		if block.section != "server" {
			continue
		}
		directives := block.directives
		for i := 0; i < len(directives); i++ {
			d := directives[i]
			if d.key != "local-zone" {
				continue
			}
			name, zoneType, ok := parseLocalZoneValue(d.value)
			groupStart := i
			groupEnd := i + 1
			for groupEnd < len(directives) && (directives[groupEnd].key == "local-data" || directives[groupEnd].key == "local-data-ptr") {
				groupEnd++
			}
			i = groupEnd - 1 // outer loop's i++ resumes right after the group

			if !ok || zoneType != "static" || validateCanonicalZoneName(name) != nil {
				continue
			}
			if _, isReserved := reserved[name]; isReserved {
				continue
			}
			zone, usedIndexes, clean := buildLocalZoneCandidate(name, directives[groupStart:groupEnd], groupStart)
			if !clean {
				continue
			}
			zones = append(zones, zone)
			if consumed[blockIndex] == nil {
				consumed[blockIndex] = map[int]bool{}
			}
			for _, idx := range usedIndexes {
				consumed[blockIndex][idx] = true
			}
			findings = append(findings, ImportFinding{
				Section: "server", Line: d.line, Directive: "local-zone", Value: name,
				Disposition: ImportGuided, Detail: "applied as a guided local host inventory zone",
			})
		}
	}
	return zones, consumed, findings
}

// validCanonicalLocalAddress mirrors the structural checks in
// validateLocalHostAddress (canonical address, correct family, not
// unspecified/loopback/multicast) - checked up front for the same reason as
// validForwardTarget above. Cross-zone PTR-address uniqueness is a
// candidate-wide concern already re-checked by the normal validate/preview
// lifecycle once this result is submitted, so it's deliberately not
// duplicated here.
func validCanonicalLocalAddress(value string, wantIPv4 bool) bool {
	address, err := netip.ParseAddr(value)
	if err != nil || address.String() != value {
		return false
	}
	if wantIPv4 && !address.Is4() {
		return false
	}
	if !wantIPv4 && (address.Is4() || address.Is4In6()) {
		return false
	}
	if address.IsUnspecified() || address.IsLoopback() || address.IsMulticast() {
		return false
	}
	return true
}

// buildLocalZoneCandidate turns one local-zone group (group[0] is the
// local-zone line itself) into a LocalZone. groupStart is group[0]'s index
// within its block's directive list, used to report which indexes were
// consumed.
func buildLocalZoneCandidate(zoneName string, group []parsedDirective, groupStart int) (LocalZone, []int, bool) {
	type hostState struct {
		hostname   string
		ipv4, ipv6 string
		ptr        bool
	}
	var order []string
	hosts := map[string]*hostState{}
	usedIndexes := []int{groupStart}
	dataSuffix := "." + zoneName
	ptrSuffix := "." + strings.TrimSuffix(zoneName, ".")

	for offset, d := range group[1:] {
		idx := groupStart + 1 + offset
		switch d.key {
		case "local-data":
			fields := strings.Fields(d.value)
			if len(fields) != 4 || !strings.EqualFold(fields[1], "IN") || !strings.HasSuffix(fields[0], dataSuffix) {
				return LocalZone{}, nil, false
			}
			hostname := strings.TrimSuffix(fields[0], dataSuffix)
			if hostname == "" || validateHostLabel(hostname) != nil {
				return LocalZone{}, nil, false
			}
			state, exists := hosts[hostname]
			if !exists {
				state = &hostState{hostname: hostname}
				hosts[hostname] = state
				order = append(order, hostname)
			}
			switch address := fields[3]; strings.ToUpper(fields[2]) {
			case "A":
				if state.ipv4 != "" || !validCanonicalLocalAddress(address, true) {
					return LocalZone{}, nil, false
				}
				state.ipv4 = address
			case "AAAA":
				if state.ipv6 != "" || !validCanonicalLocalAddress(address, false) {
					return LocalZone{}, nil, false
				}
				state.ipv6 = address
			default:
				// CNAME or anything else has no guided equivalent (#131).
				return LocalZone{}, nil, false
			}
			usedIndexes = append(usedIndexes, idx)
		case "local-data-ptr":
			fields := strings.Fields(d.value)
			if len(fields) != 2 || !strings.HasSuffix(fields[1], ptrSuffix) {
				return LocalZone{}, nil, false
			}
			address := fields[0]
			hostname := strings.TrimSuffix(fields[1], ptrSuffix)
			state, exists := hosts[hostname]
			if !exists || (state.ipv4 != address && state.ipv6 != address) {
				return LocalZone{}, nil, false
			}
			state.ptr = true
			usedIndexes = append(usedIndexes, idx)
		}
	}

	if len(order) == 0 {
		return LocalZone{}, nil, false
	}
	zone := LocalZone{Name: zoneName}
	for _, hostname := range order {
		state := hosts[hostname]
		zone.Hosts = append(zone.Hosts, LocalHost{Hostname: state.hostname, IPv4: state.ipv4, IPv6: state.ipv6, PTR: state.ptr})
	}
	return zone, usedIndexes, true
}

// parseLocalZoneValue splits a local-zone directive's raw value - a quoted
// zone name followed by an unquoted type keyword, e.g. `"home.lab." static`
// - which unquoteDirectiveValue (a single-token unquoter) leaves untouched.
func parseLocalZoneValue(value string) (name, zoneType string, ok bool) {
	if !strings.HasPrefix(value, `"`) {
		return "", "", false
	}
	closeIdx := strings.Index(value[1:], `"`)
	if closeIdx < 0 {
		return "", "", false
	}
	name = value[1 : 1+closeIdx]
	zoneType = strings.TrimSpace(value[1+closeIdx+1:])
	if zoneType == "" {
		return "", "", false
	}
	return name, zoneType, true
}

// mergeConsumedDirectives folds src's consumed directive indexes into dst.
func mergeConsumedDirectives(dst, src map[int]map[int]bool) {
	for blockIndex, indexes := range src {
		if dst[blockIndex] == nil {
			dst[blockIndex] = map[int]bool{}
		}
		for index := range indexes {
			dst[blockIndex][index] = true
		}
	}
}

// extractReverseZonePolicies reverse-maps the RFC1918 reverse-zone local-zone
// lines Settings.Render() itself generates (rfc1918ReverseZones) onto the
// three ReverseZonePolicy entries every Settings already carries. A network
// only maps cleanly when EVERY one of its reverse-zone names appears exactly
// once, all with the same local-zone type ("static" or "transparent",
// reverseZoneType's own vocabulary) - a partial set, a duplicate, an
// inconsistent type, or an unrecognized type keyword all leave those lines
// unconsumed rather than guessing at a policy that wasn't actually stated
// cleanly.
func extractReverseZonePolicies(blocks []parsedBlock) (policies map[string]string, consumed map[int]map[int]bool, findings []ImportFinding) {
	policies = map[string]string{}
	consumed = map[int]map[int]bool{}

	type hit struct {
		blockIndex, directiveIndex, line int
		zoneType                         string
	}
	zoneNameToNetwork := map[string]string{}
	for network, names := range rfc1918ReverseZones {
		for _, name := range names {
			zoneNameToNetwork[name] = network
		}
	}
	hitsByNetworkAndName := map[string]map[string][]hit{}
	for blockIndex, block := range blocks {
		if block.section != "server" {
			continue
		}
		for directiveIndex, d := range block.directives {
			if d.key != "local-zone" {
				continue
			}
			name, zoneType, ok := parseLocalZoneValue(d.value)
			if !ok {
				continue
			}
			network, isReserved := zoneNameToNetwork[name]
			if !isReserved {
				continue
			}
			if hitsByNetworkAndName[network] == nil {
				hitsByNetworkAndName[network] = map[string][]hit{}
			}
			hitsByNetworkAndName[network][name] = append(hitsByNetworkAndName[network][name], hit{blockIndex, directiveIndex, d.line, zoneType})
		}
	}

	for network, expectedNames := range rfc1918ReverseZones {
		byName := hitsByNetworkAndName[network]
		if len(byName) != len(expectedNames) {
			continue
		}
		var matched []hit
		var zoneType string
		clean := true
		for _, name := range expectedNames {
			occurrences, ok := byName[name]
			if !ok || len(occurrences) != 1 {
				clean = false
				break
			}
			if zoneType == "" {
				zoneType = occurrences[0].zoneType
			} else if occurrences[0].zoneType != zoneType {
				clean = false
				break
			}
			matched = append(matched, occurrences[0])
		}
		var mode string
		switch {
		case !clean:
			continue
		case zoneType == "static":
			mode = reverseModeNXDOMAIN
		case zoneType == "transparent":
			mode = reverseModeTransparent
		default:
			continue
		}
		policies[network] = mode
		for _, h := range matched {
			if consumed[h.blockIndex] == nil {
				consumed[h.blockIndex] = map[int]bool{}
			}
			consumed[h.blockIndex][h.directiveIndex] = true
			findings = append(findings, ImportFinding{
				Section: "server", Line: h.line, Directive: "local-zone", Value: network,
				Disposition: ImportGuided,
				Detail:      fmt.Sprintf("applied as part of the guided RFC1918 reverse-zone policy for %s (%s)", network, mode),
			})
		}
	}
	return policies, consumed, findings
}

// extractNetworkMode reverse-maps do-ip4/do-ip6/prefer-ip6 onto NetworkMode
// only when all three appear exactly once with one of the three combinations
// Settings.Render() itself produces - any other combination (missing,
// duplicated, or simply not a combination Render() would generate) is left
// for guidedNotYetMapped's blocked fallback rather than guessed at.
func extractNetworkMode(blocks []parsedBlock) (mode string, consumed map[int]map[int]bool, findings []ImportFinding) {
	consumed = map[int]map[int]bool{}
	type hit struct {
		blockIndex, directiveIndex, line int
		value                            string
	}
	hits := map[string][]hit{}
	for blockIndex, block := range blocks {
		if block.section != "server" {
			continue
		}
		for directiveIndex, d := range block.directives {
			if d.key == "do-ip4" || d.key == "do-ip6" || d.key == "prefer-ip6" {
				hits[d.key] = append(hits[d.key], hit{blockIndex, directiveIndex, d.line, strings.ToLower(d.value)})
			}
		}
	}
	if len(hits["do-ip4"]) != 1 || len(hits["do-ip6"]) != 1 || len(hits["prefer-ip6"]) != 1 {
		return "", consumed, nil
	}
	ip4, ip6, prefer6 := hits["do-ip4"][0], hits["do-ip6"][0], hits["prefer-ip6"][0]
	switch {
	case ip4.value == "yes" && ip6.value == "no" && prefer6.value == "no":
		mode = networkModeIPv4
	case ip4.value == "yes" && ip6.value == "yes" && prefer6.value == "no":
		mode = networkModeDual
	case ip4.value == "no" && ip6.value == "yes" && prefer6.value == "yes":
		mode = networkModeIPv6
	default:
		return "", consumed, nil
	}
	for _, item := range []struct {
		key string
		h   hit
	}{{"do-ip4", ip4}, {"do-ip6", ip6}, {"prefer-ip6", prefer6}} {
		if consumed[item.h.blockIndex] == nil {
			consumed[item.h.blockIndex] = map[int]bool{}
		}
		consumed[item.h.blockIndex][item.h.directiveIndex] = true
		findings = append(findings, ImportFinding{
			Section: "server", Line: item.h.line, Directive: item.key, Value: item.h.value,
			Disposition: ImportGuided, Detail: fmt.Sprintf("applied as part of the guided network mode (%s)", mode),
		})
	}
	return mode, consumed, findings
}

// extractResourceProfile reverse-maps rrset-cache-size/msg-cache-size onto
// ResourceProfile only when both appear exactly once and match one of
// RootGuard's three preset size pairs (resourceProfileCacheSizes) exactly -
// any other pairing is left for guidedNotYetMapped's blocked fallback.
func extractResourceProfile(blocks []parsedBlock) (profile string, consumed map[int]map[int]bool, findings []ImportFinding) {
	consumed = map[int]map[int]bool{}
	type hit struct {
		blockIndex, directiveIndex, line int
		value                            string
	}
	hits := map[string][]hit{}
	for blockIndex, block := range blocks {
		if block.section != "server" {
			continue
		}
		for directiveIndex, d := range block.directives {
			if d.key == "rrset-cache-size" || d.key == "msg-cache-size" {
				hits[d.key] = append(hits[d.key], hit{blockIndex, directiveIndex, d.line, d.value})
			}
		}
	}
	if len(hits["rrset-cache-size"]) != 1 || len(hits["msg-cache-size"]) != 1 {
		return "", consumed, nil
	}
	rrset, msg := hits["rrset-cache-size"][0], hits["msg-cache-size"][0]
	for name, sizes := range resourceProfileCacheSizes {
		if sizes.RRSet == rrset.value && sizes.Message == msg.value {
			profile = name
			break
		}
	}
	if profile == "" {
		return "", consumed, nil
	}
	for _, item := range []struct {
		key string
		h   hit
	}{{"rrset-cache-size", rrset}, {"msg-cache-size", msg}} {
		if consumed[item.h.blockIndex] == nil {
			consumed[item.h.blockIndex] = map[int]bool{}
		}
		consumed[item.h.blockIndex][item.h.directiveIndex] = true
		findings = append(findings, ImportFinding{
			Section: "server", Line: item.h.line, Directive: item.key, Value: item.h.value,
			Disposition: ImportGuided, Detail: fmt.Sprintf("applied as the guided resource profile (%s)", profile),
		})
	}
	return profile, consumed, findings
}

// classifyZoneScopedDirective resolves the ambiguity that domain-insecure
// and private-domain share in RootGuard's own Render(): a private-domain
// line is a global guided private domain UNLESS its value names one of the
// forward zones just extracted, in which case both directives are that
// zone's AllowUnsigned/AllowPrivateAddresses opt-in instead (see
// Settings.Render()). A domain-insecure with no matching zone has no guided
// meaning at all outside that context, so it's offered for expert adoption
// rather than silently dropped.
func classifyZoneScopedDirective(zones []ForwardZone, zoneNames map[string]struct{}, d parsedDirective) (finding ImportFinding, appendedPrivateDomain string) {
	base := ImportFinding{Section: "server", Line: d.line, Directive: d.key, Value: d.value}
	if _, isZone := zoneNames[d.value]; isZone {
		for i := range zones {
			if zones[i].Name != d.value {
				continue
			}
			if d.key == "domain-insecure" {
				zones[i].AllowUnsigned = true
			} else {
				zones[i].AllowPrivateAddresses = true
			}
		}
		base.Disposition = ImportGuided
		base.Detail = fmt.Sprintf("applied as an opt-in for the %q forwarding zone", d.value)
		return base, ""
	}
	if d.key == "private-domain" {
		base.Disposition = ImportGuided
		base.Detail = "applied to the guided private-domain list"
		return base, d.value
	}
	base.Disposition = ImportExpert
	base.Detail = "does not match an imported forwarding zone - no guided equivalent, offered for expert adoption"
	return base, ""
}

func classifyServerDirective(candidate *Settings, d parsedDirective) ImportFinding {
	base := ImportFinding{Section: "server", Line: d.line, Directive: d.key, Value: d.value}

	switch d.key {
	case "qname-minimisation":
		return applyGuidedBool(base, &candidate.QnameMinimisation, d.value)
	case "prefetch":
		return applyGuidedBool(base, &candidate.Prefetch, d.value)
	case "prefetch-key":
		return applyGuidedBool(base, &candidate.PrefetchKey, d.value)
	case "aggressive-nsec":
		return applyGuidedBool(base, &candidate.AggressiveNSEC, d.value)
	case "serve-expired":
		return applyGuidedBool(base, &candidate.ServeExpired, d.value)
	case "edns-buffer-size":
		return applyGuidedInt(base, &candidate.EDNSBufferSize, d.value)
	case "verbosity":
		return applyGuidedInt(base, &candidate.LogVerbosity, d.value)
	case "serve-expired-ttl":
		return applyGuidedInt(base, &candidate.ServeExpiredTTL, d.value)
	case "serve-expired-client-timeout":
		return applyGuidedInt(base, &candidate.ServeExpiredClientTimeout, d.value)
	case "cache-min-ttl":
		return applyGuidedInt(base, &candidate.CacheMinTTL, d.value)
	case "cache-max-ttl":
		return applyGuidedInt(base, &candidate.CacheMaxTTL, d.value)
	case "num-threads":
		return applyGuidedInt(base, &candidate.Threads, d.value)
	}

	if expected, ok := fixedBaseScalars[d.key]; ok {
		base.Disposition = ImportFixedBase
		if d.value != expected {
			base.Disposition = ImportBlocked
			base.Detail = fmt.Sprintf("conflicts with the RootGuard fixed base, which sets this to %q", expected)
		} else {
			base.Detail = "already set by the RootGuard fixed base"
		}
		return base
	}
	if _, ok := fixedBaseStructural[d.key]; ok {
		base.Disposition = ImportFixedBase
		base.Detail = "managed by the RootGuard Unbound image"
		return base
	}
	if detail, ok := guidedNotYetMapped[d.key]; ok {
		base.Disposition = ImportBlocked
		base.Detail = detail
		return base
	}
	if reason, blocked := blockedDirectives[d.key]; blocked {
		base.Disposition = ImportBlocked
		base.Detail = reason
		return base
	}
	base.Disposition = ImportExpert
	base.Detail = "no guided equivalent - offered for expert adoption"
	return base
}

func applyGuidedBool(base ImportFinding, field *bool, value string) ImportFinding {
	switch strings.ToLower(value) {
	case "yes":
		*field = true
	case "no":
		*field = false
	default:
		base.Disposition = ImportBlocked
		base.Detail = fmt.Sprintf("%q is not a valid yes/no value", value)
		return base
	}
	base.Disposition = ImportGuided
	base.Detail = "applied to the guided setting"
	return base
}

func applyGuidedInt(base ImportFinding, field *int, value string) ImportFinding {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		base.Disposition = ImportBlocked
		base.Detail = fmt.Sprintf("%q is not a valid integer", value)
		return base
	}
	*field = parsed
	base.Disposition = ImportGuided
	base.Detail = "applied to the guided setting"
	return base
}

// parseUnboundConf splits content into clause blocks. Unbound's own grammar
// is indentation-independent: a line whose value is empty after the colon
// (e.g. "server:", "forward-zone:") opens a new clause that lasts until the
// next such line or EOF; every other "key: value" line is a directive within
// the clause currently open (defaulting to "server" before any explicit
// clause header, which is also valid unbound.conf).
func parseUnboundConf(content string) []parsedBlock {
	var blocks []parsedBlock
	current := parsedBlock{section: "server", lineStart: 1}
	flush := func() {
		if len(current.directives) > 0 || current.section != "server" {
			blocks = append(blocks, current)
		}
	}
	for lineNumber, raw := range strings.Split(content, "\n") {
		line := raw
		if idx := strings.Index(line, "#"); idx >= 0 {
			line = line[:idx]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, rawValue, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		rawValue = strings.TrimSpace(rawValue)
		// A line with genuinely nothing after the colon opens a new clause
		// (e.g. "server:"). A quoted empty string ("chroot: \"\"") is a real
		// directive with an empty value, not a clause header, so this check
		// must happen before unquoting collapses both cases to "".
		if rawValue == "" {
			flush()
			current = parsedBlock{section: key, lineStart: lineNumber + 1}
			continue
		}
		current.directives = append(current.directives, parsedDirective{
			key: key, value: unquoteDirectiveValue(rawValue), line: lineNumber + 1, raw: strings.TrimSpace(raw),
		})
	}
	flush()
	return blocks
}

func unquoteDirectiveValue(value string) string {
	if len(value) >= 2 && strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`) {
		return value[1 : len(value)-1]
	}
	return value
}
