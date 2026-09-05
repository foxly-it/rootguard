package unbound

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomPreviewValidatesWithoutChangingActiveFile(t *testing.T) {
	manager := newTestManager(t)
	content := "server:\n    hide-identity: yes\n"
	preview, err := manager.PreviewCustom(context.Background(), content)
	if err != nil {
		t.Fatal(err)
	}
	if !preview.Changed || preview.Validation == "" || len(preview.Advice) == 0 {
		t.Fatalf("unexpected preview: %+v", preview)
	}
	if _, err := os.Stat(filepath.Join(manager.hostConfigDir, "90-rootguard-custom.conf")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("preview changed active config: %v", err)
	}
}

func TestCustomConfigIsActivatedVersionedAndRestored(t *testing.T) {
	manager := newTestManager(t)
	first := "server:\n    hide-identity: yes\n"
	second := "server:\n    hide-version: yes\n"
	if _, err := manager.ApplyCustom(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.ApplyCustom(context.Background(), second); err != nil {
		t.Fatal(err)
	}
	history, err := manager.History()
	if err != nil {
		t.Fatal(err)
	}
	var firstID string
	for _, entry := range history {
		if entry.CustomConfig == first {
			firstID = entry.ID
		}
	}
	if firstID == "" {
		t.Fatalf("first custom version not recorded: %+v", history)
	}
	if _, err := manager.Restore(context.Background(), firstID); err != nil {
		t.Fatal(err)
	}
	document, err := manager.LoadCustom()
	if err != nil {
		t.Fatal(err)
	}
	if document.Content != first {
		t.Fatalf("unexpected restored custom config: %q", document.Content)
	}
}

func TestCustomConfigRejectsManagedAndDangerousDirectives(t *testing.T) {
	for _, content := range []string{
		"server:\n    num-threads: 8\n",
		"include: \"/etc/passwd\"\n",
		"server:\n    interface: 0.0.0.0\n",
	} {
		if _, err := normalizeCustom(content); !errors.Is(err, ErrInvalidCustomConfig) {
			t.Fatalf("expected policy rejection for %q, got %v", content, err)
		}
	}
}

// TestCustomConfigRejectsDNSSECBypasses is the regression test for a real
// gap found in review: docs.html claims the expert editor "blocks...
// DNSSEC bypasses", but domain-insecure was completely unrestricted (a
// bare `domain-insecure: "."` disables DNSSEC validation for the entire
// namespace, not just one zone) and harden-dnssec-stripped: no only
// produced a soft "warning" advisory a user could activate anyway.
func TestCustomConfigRejectsDNSSECBypasses(t *testing.T) {
	for _, content := range []string{
		"server:\n    domain-insecure: \".\"\n",
		"server:\n    domain-insecure: \"example.internal\"\n",
		"server:\n    harden-dnssec-stripped: no\n",
		// Regression cases found in review: the old check matched a
		// literal ": no" suffix against the raw line, which missed every
		// one of these - all ordinary, spec-legal Unbound config shapes.
		"server:\n    harden-dnssec-stripped:    no\n",         // extra internal whitespace
		"server:\n    harden-dnssec-stripped: no # allow it\n", // trailing comment
		"server:\n    harden-dnssec-stripped:no\n",             // no space after the colon
		// Not itself a regression (the old whole-line lowercase already
		// covered this) - kept as a companion case now that the value
		// comparison is its own explicit EqualFold rather than inherited
		// from a whole-line lowercase.
		"server:\n    HARDEN-DNSSEC-STRIPPED: NO\n",
		// Regression cases found in a follow-up review: Unbound's own
		// lexer strips a single matching layer of double or single quotes
		// from a directive value before the parser ever sees it, so both
		// of these are ordinary, spec-legal ways to write "no" too - the
		// EqualFold comparison above never unquoted the value, so neither
		// was recognized as the bypass it actually is.
		"server:\n    harden-dnssec-stripped: \"no\"\n",
		"server:\n    harden-dnssec-stripped: 'no'\n",
		// Anything that isn't unambiguously "yes" once unquoted must also
		// be refused for this directive specifically (see the whitelist
		// comment in normalizeCustom) - not just the known "no" spelling.
		"server:\n    harden-dnssec-stripped: maybe\n",
	} {
		if _, err := normalizeCustom(content); !errors.Is(err, ErrInvalidCustomConfig) {
			t.Fatalf("expected policy rejection for %q, got %v", content, err)
		}
	}
	// The recommended, secure value must still be accepted - only "no"
	// weakens DNSSEC, "yes" (the default DirectiveReferences itself
	// recommends) must not be blocked outright. Quoted spellings of "yes"
	// must be accepted too, for the same unquoting reason as above.
	for _, content := range []string{
		"server:\n    harden-dnssec-stripped: yes\n",
		"server:\n    harden-dnssec-stripped: \"yes\"\n",
		"server:\n    harden-dnssec-stripped: 'yes'\n",
	} {
		if _, err := normalizeCustom(content); err != nil {
			t.Fatalf("harden-dnssec-stripped: yes (quoted or not) must remain accepted, got %v for %q", err, content)
		}
	}
}

// TestCustomConfigRejectsPublicAccessControl is the regression test for a
// real gap found in review: access-control only ever produced a
// dismissible "warning" advisory, even for a rule that turns the resolver
// into an internet-reachable open resolver (access-control: 0.0.0.0/0
// allow, combined with the installer's own permitted DNSBindAddress:
// 0.0.0.0) - a classic DNS-amplification setup. DirectiveReferences
// itself already documents access-control as "Risk: high".
func TestCustomConfigRejectsPublicAccessControl(t *testing.T) {
	for _, content := range []string{
		"server:\n    access-control: 0.0.0.0/0 allow\n",
		"server:\n    access-control: ::/0 allow\n",
		// A single public host, not just a whole-internet range.
		"server:\n    access-control: 8.8.8.8/32 allow\n",
		"server:\n    access-control: 8.8.8.8 allow\n",
		// Every allow-family action grants access, not just plain "allow".
		"server:\n    access-control: 0.0.0.0/0 allow_snoop\n",
		"server:\n    access-control: 0.0.0.0/0 allow_setrd\n",
		"server:\n    access-control: 0.0.0.0/0 allow_cookie\n",
		// A range partially outside RFC1918 must still be refused - it's
		// not enough for the range to merely overlap a private one.
		"server:\n    access-control: 192.168.0.0/8 allow\n",
		// Unparseable ranges fail closed rather than being let through.
		"server:\n    access-control: not-an-ip allow\n",
	} {
		if _, err := normalizeCustom(content); !errors.Is(err, ErrInvalidCustomConfig) {
			t.Fatalf("expected policy rejection for %q, got %v", content, err)
		}
	}
	// Legitimate LAN-range allow rules, and any range under a
	// restricting action, must remain usable - this is a hard block on
	// reaching beyond private space, not on the directive itself.
	for _, content := range []string{
		"server:\n    access-control: 192.168.1.0/24 allow\n",
		"server:\n    access-control: 10.0.0.0/8 allow\n",
		"server:\n    access-control: 127.0.0.1/32 allow\n",
		"server:\n    access-control: ::1/128 allow\n",
		"server:\n    access-control: fe80::/10 allow\n",
		// deny/refuse/*_non_local only ever restrict access - never
		// blocked regardless of range.
		"server:\n    access-control: 0.0.0.0/0 refuse\n",
		"server:\n    access-control: 0.0.0.0/0 deny\n",
		"server:\n    access-control: 0.0.0.0/0 deny_non_local\n",
	} {
		if _, err := normalizeCustom(content); err != nil {
			t.Fatalf("expected %q to remain accepted, got %v", content, err)
		}
	}
}

func TestCustomConfigRestoresFilesWhenEffectiveCheckFails(t *testing.T) {
	manager := newTestManager(t)
	initial := "server:\n    hide-identity: yes\n"
	if _, err := manager.ApplyCustom(context.Background(), initial); err != nil {
		t.Fatal(err)
	}
	manager.run = func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if strings.Contains(strings.Join(args, " "), "/etc/unbound/unbound.conf") {
			return []byte("duplicate or invalid option"), errors.New("exit 1")
		}
		return []byte("OK"), nil
	}
	if _, err := manager.ApplyCustom(context.Background(), "server:\n    hide-version: yes\n"); err == nil {
		t.Fatal("expected effective configuration failure")
	}
	document, err := manager.LoadCustom()
	if err != nil {
		t.Fatal(err)
	}
	if document.Content != initial {
		t.Fatalf("failed validation left candidate active: %q", document.Content)
	}
}

func TestCustomAdvisorFlagsHardeningAndForwarding(t *testing.T) {
	advice := adviseCustom("server:\n    hide-version: no\nforward-zone:\n    name: \".\"\n    forward-addr: 1.1.1.1\n")
	foundWarning, foundForwarder := false, false
	for _, item := range advice {
		foundWarning = foundWarning || item.Severity == "warning"
		foundForwarder = foundForwarder || strings.HasPrefix(item.ID, "forwarding-")
	}
	if !foundWarning || !foundForwarder {
		t.Fatalf("expected hardening and forwarding advice: %+v", advice)
	}
}

func TestGuidedForwardingRejectsExpertForwardZoneConflict(t *testing.T) {
	manager := newTestManager(t)
	if _, err := manager.ApplyCustom(context.Background(), "forward-zone:\n    name: \"legacy.example.\"\n    forward-addr: 192.0.2.53\n"); err != nil {
		t.Fatal(err)
	}

	settings := DefaultSettings()
	settings.ForwardZones = []ForwardZone{{
		Name:    "corp.example.",
		Servers: []string{"192.0.2.54"},
	}}
	if _, err := manager.Preview(settings); !errors.Is(err, ErrInvalidCustomConfig) {
		t.Fatalf("expected guided/expert conflict, got %v", err)
	}
	if err := manager.Apply(context.Background(), settings); !errors.Is(err, ErrInvalidCustomConfig) {
		t.Fatalf("expected apply conflict, got %v", err)
	}
}

func TestGuidedPrivateSettingsRejectExpertDirectiveConflicts(t *testing.T) {
	manager := newTestManager(t)
	if _, err := manager.ApplyCustom(context.Background(), "server:\n    private-domain: \"legacy.example.\"\n"); err != nil {
		t.Fatal(err)
	}
	settings := DefaultSettings()
	settings.PrivateDomains = []string{"home.example."}
	if _, err := manager.Preview(settings); !errors.Is(err, ErrInvalidCustomConfig) {
		t.Fatalf("expected private-domain conflict, got %v", err)
	}

	manager = newTestManager(t)
	if _, err := manager.ApplyCustom(context.Background(), "server:\n    local-zone: \"168.192.in-addr.arpa.\" transparent\n"); !errors.Is(err, ErrInvalidCustomConfig) {
		t.Fatalf("expected guided reverse-zone ownership conflict, got %v", err)
	}
}

func TestGuidedLocalHostInventoryRejectsExpertLocalZoneConflict(t *testing.T) {
	manager := newTestManager(t)
	settings := DefaultSettings()
	settings.LocalZones = []LocalZone{{
		Name:  "home.lab.",
		Hosts: []LocalHost{{Hostname: "printer", IPv4: "192.168.1.20"}},
	}}
	if err := manager.Apply(context.Background(), settings); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.ApplyCustom(context.Background(), "server:\n    local-zone: \"home.lab.\" transparent\n"); !errors.Is(err, ErrInvalidCustomConfig) {
		t.Fatalf("expected guided local host inventory conflict, got %v", err)
	}
}
