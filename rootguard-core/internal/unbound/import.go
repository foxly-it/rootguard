package unbound

import (
	"fmt"
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

// guidedNotYetMapped are directives that DO belong to a guided setting but
// that this importer doesn't reverse-map yet (network mode is a three-
// directive combination, forward/local zones are block-shaped). Classified
// as blocked rather than expert so they don't silently duplicate a guided
// concept in the custom config, where they'd be rejected anyway by
// blockedDirectives - the point is an accurate up-front finding, not a
// deferred failure.
var guidedNotYetMapped = map[string]string{
	"do-ip4":           "maps to the guided network mode, not yet supported by this importer - set it directly in the guided form",
	"do-ip6":           "maps to the guided network mode, not yet supported by this importer - set it directly in the guided form",
	"prefer-ip6":       "maps to the guided network mode, not yet supported by this importer - set it directly in the guided form",
	"rrset-cache-size": "maps to the guided resource profile, not yet supported by this importer - set it directly in the guided form",
	"msg-cache-size":   "maps to the guided resource profile, not yet supported by this importer - set it directly in the guided form",
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
			// No guided mapping yet for this clause (local-zone, stub-zone,
			// view, ... - or a forward-zone that didn't map cleanly) - offer
			// it for expert adoption as a whole, exactly as if it had been
			// pasted into the expert editor by hand.
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
		for _, d := range block.directives {
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
	if len(privateDomains) > 0 {
		candidate.PrivateDomains = append(append([]string{}, candidate.PrivateDomains...), privateDomains...)
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
		if !clean || zone.Name == "" || len(zone.Servers) == 0 {
			// Not left in mappedBlocks, so the caller's main loop falls back
			// to its generic whole-block expert-adoption handling for this
			// block - do not also emit a finding for it here.
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
