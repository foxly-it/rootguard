package unbound

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestDefaultSettingsRender(t *testing.T) {
	config, err := DefaultSettings().Render()
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"# Privacy:",
		"qname-minimisation: yes",
		"# Performance:",
		"prefetch: yes",
		"# Availability:",
		"serve-expired: yes",
		"cache-max-ttl: 86400",
		"num-threads: 2",
	} {
		if !strings.Contains(string(config), expected) {
			t.Errorf("rendered config does not contain %q", expected)
		}
	}
}

func TestActiveConfigurationReadsRunningContainerFiles(t *testing.T) {
	manager := NewManager(t.TempDir(), "/etc/unbound/unbound.d", "rootguard-unbound")
	manager.now = func() time.Time { return time.Unix(123, 0) }
	manager.run = func(_ context.Context, name string, arguments ...string) ([]byte, error) {
		if name != "docker" || len(arguments) != 4 || arguments[0] != "exec" || arguments[2] != "cat" {
			t.Fatalf("unexpected command: %s %v", name, arguments)
		}
		switch arguments[3] {
		case "/etc/unbound/unbound.conf":
			return []byte("include: /etc/unbound/unbound.d/*.conf\n"), nil
		case "/etc/unbound/unbound.d/50-rootguard.conf":
			return []byte("server:\n    prefetch: yes\n"), nil
		default:
			t.Fatalf("unexpected path: %s", arguments[3])
			return nil, nil
		}
	}

	active, err := manager.ActiveConfiguration(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(active.BaseConfig, "include:") || !strings.Contains(active.ManagedConfig, "prefetch: yes") {
		t.Fatalf("unexpected active configuration: %#v", active)
	}
	if active.CustomConfig != "" || !active.CheckedAt.Equal(time.Unix(123, 0)) {
		t.Fatalf("unexpected active metadata: %#v", active)
	}
}

func TestSettingsValidation(t *testing.T) {
	settings := DefaultSettings()
	settings.Threads = 0
	if err := settings.Validate(); !errors.Is(err, ErrInvalidSettings) {
		t.Fatalf("expected ErrInvalidSettings, got %v", err)
	}
}

func TestForwardZonesRenderCanonicalOrderedTargets(t *testing.T) {
	settings := DefaultSettings()
	settings.ForwardZones = []ForwardZone{
		{
			Name:                  "corp.example.",
			Servers:               []string{"192.0.2.53", "2001:db8::53"},
			ForwardFirst:          true,
			AllowUnsigned:         true,
			AllowPrivateAddresses: true,
		},
	}
	config, err := settings.Render()
	if err != nil {
		t.Fatal(err)
	}
	rendered := string(config)
	for _, expected := range []string{
		"# Conditional forwarding:",
		"forward-zone:",
		`name: "corp.example."`,
		"forward-addr: 192.0.2.53",
		"forward-addr: 2001:db8::53",
		"forward-first: yes",
		`domain-insecure: "corp.example."`,
		`private-domain: "corp.example."`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Errorf("rendered forwarding config does not contain %q", expected)
		}
	}
	if strings.Index(rendered, "192.0.2.53") > strings.Index(rendered, "2001:db8::53") {
		t.Fatal("forward target order was not preserved")
	}
	if strings.Index(rendered, `domain-insecure: "corp.example."`) > strings.Index(rendered, "forward-zone:") {
		t.Fatal("domain-insecure must be rendered inside the server section")
	}
	if strings.Index(rendered, `private-domain: "corp.example."`) > strings.Index(rendered, "forward-zone:") {
		t.Fatal("private-domain must be rendered inside the server section")
	}
}

func TestForwardZonesKeepDNSSECValidationByDefault(t *testing.T) {
	settings := DefaultSettings()
	settings.ForwardZones = []ForwardZone{{
		Name:    "corp.example.",
		Servers: []string{"192.0.2.53"},
	}}
	config, err := settings.Render()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(config), "domain-insecure:") {
		t.Fatal("unsigned zones must require explicit opt-in")
	}
	if strings.Contains(string(config), "private-domain:") {
		t.Fatal("private-address answers must require explicit opt-in")
	}
}

func TestForwardZoneValidationRejectsUnsafeInputs(t *testing.T) {
	tests := []struct {
		name string
		zone ForwardZone
	}{
		{name: "root zone", zone: ForwardZone{Name: ".", Servers: []string{"192.0.2.53"}}},
		{name: "non canonical name", zone: ForwardZone{Name: "Corp.Example", Servers: []string{"192.0.2.53"}}},
		{name: "invalid label", zone: ForwardZone{Name: "-corp.example.", Servers: []string{"192.0.2.53"}}},
		{name: "missing target", zone: ForwardZone{Name: "corp.example."}},
		{name: "non canonical IPv6", zone: ForwardZone{Name: "corp.example.", Servers: []string{"2001:0db8::53"}}},
		{name: "loopback target", zone: ForwardZone{Name: "corp.example.", Servers: []string{"127.0.0.1"}}},
		{name: "RootGuard network target", zone: ForwardZone{Name: "corp.example.", Servers: []string{"172.29.53.3"}}},
		{name: "mapped RootGuard target", zone: ForwardZone{Name: "corp.example.", Servers: []string{"::ffff:172.29.53.3"}}},
		{name: "link-local target", zone: ForwardZone{Name: "corp.example.", Servers: []string{"fe80::53"}}},
		{name: "duplicate target", zone: ForwardZone{Name: "corp.example.", Servers: []string{"192.0.2.53", "192.0.2.53"}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			settings := DefaultSettings()
			settings.ForwardZones = []ForwardZone{test.zone}
			if err := settings.Validate(); !errors.Is(err, ErrInvalidSettings) {
				t.Fatalf("expected ErrInvalidSettings, got %v", err)
			}
		})
	}
}

func TestForwardZoneValidationRejectsDuplicateZonesAndLimits(t *testing.T) {
	settings := DefaultSettings()
	settings.ForwardZones = []ForwardZone{
		{Name: "corp.example.", Servers: []string{"192.0.2.53"}},
		{Name: "corp.example.", Servers: []string{"192.0.2.54"}},
	}
	if err := settings.Validate(); !errors.Is(err, ErrInvalidSettings) {
		t.Fatalf("expected duplicate zone rejection, got %v", err)
	}

	settings.ForwardZones = make([]ForwardZone, maxForwardZones+1)
	if err := settings.Validate(); !errors.Is(err, ErrInvalidSettings) {
		t.Fatalf("expected zone limit rejection, got %v", err)
	}
}

func TestPrivateDomainsAndRFC1918ReversePoliciesRender(t *testing.T) {
	settings := DefaultSettings()
	settings.PrivateDomains = []string{"home.example."}
	settings.ReverseZones = []ReverseZonePolicy{
		{Network: "10.0.0.0/8", Mode: reverseModeNXDOMAIN},
		{Network: "172.16.0.0/12", Mode: reverseModeTransparent},
		{Network: "192.168.0.0/16", Mode: reverseModeNXDOMAIN},
	}
	config, err := settings.Render()
	if err != nil {
		t.Fatal(err)
	}
	rendered := string(config)
	for _, expected := range []string{
		`private-domain: "home.example."`,
		`local-zone: "10.in-addr.arpa." static`,
		`local-zone: "16.172.in-addr.arpa." transparent`,
		`local-zone: "31.172.in-addr.arpa." transparent`,
		`local-zone: "168.192.in-addr.arpa." static`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Errorf("rendered private network config does not contain %q", expected)
		}
	}
}

func TestPrivateDomainValidationRejectsUnsafeAndDuplicateInputs(t *testing.T) {
	tests := []Settings{
		func() Settings {
			settings := DefaultSettings()
			settings.PrivateDomains = []string{"Home.Example"}
			return settings
		}(),
		func() Settings {
			settings := DefaultSettings()
			settings.PrivateDomains = []string{"home.example.", "home.example."}
			return settings
		}(),
		func() Settings {
			settings := DefaultSettings()
			settings.PrivateDomains = []string{"home.example."}
			settings.ForwardZones = []ForwardZone{{
				Name: "home.example.", Servers: []string{"192.0.2.53"}, AllowPrivateAddresses: true,
			}}
			return settings
		}(),
	}
	for _, settings := range tests {
		if err := settings.Validate(); !errors.Is(err, ErrInvalidSettings) {
			t.Fatalf("expected invalid private domain settings, got %v", err)
		}
	}
}

func TestRFC1918ReversePolicyValidationRejectsUnknownRangesAndModes(t *testing.T) {
	tests := []ReverseZonePolicy{
		{Network: "192.0.2.0/24", Mode: reverseModeNXDOMAIN},
		{Network: "10.0.0.0/8", Mode: "forward"},
	}
	for _, policy := range tests {
		settings := DefaultSettings()
		settings.ReverseZones = []ReverseZonePolicy{policy}
		if err := settings.Validate(); !errors.Is(err, ErrInvalidSettings) {
			t.Fatalf("expected invalid reverse policy %#v, got %v", policy, err)
		}
	}
	settings := DefaultSettings()
	settings.ReverseZones = []ReverseZonePolicy{
		{Network: "10.0.0.0/8", Mode: reverseModeNXDOMAIN},
		{Network: "10.0.0.0/8", Mode: reverseModeTransparent},
	}
	if err := settings.Validate(); !errors.Is(err, ErrInvalidSettings) {
		t.Fatalf("expected duplicate reverse network rejection, got %v", err)
	}
}
