package unbound

import (
	"errors"
	"strconv"
	"strings"
	"testing"
)

func TestLocalZonesRenderHostsAndPTR(t *testing.T) {
	settings := DefaultSettings()
	settings.LocalZones = []LocalZone{{
		Name: "home.lab.",
		Hosts: []LocalHost{
			{Hostname: "printer", IPv4: "192.168.1.20", IPv6: "2001:db8::20", PTR: true},
			{Hostname: "router", IPv4: "192.168.1.1"},
		},
	}}
	config, err := settings.Render()
	if err != nil {
		t.Fatal(err)
	}
	rendered := string(config)
	for _, expected := range []string{
		`local-zone: "home.lab." static`,
		`local-data: "printer.home.lab. IN A 192.168.1.20"`,
		`local-data: "printer.home.lab. IN AAAA 2001:db8::20"`,
		`local-data-ptr: "192.168.1.20 printer.home.lab"`,
		`local-data-ptr: "2001:db8::20 printer.home.lab"`,
		`local-data: "router.home.lab. IN A 192.168.1.1"`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Errorf("rendered local-zone config does not contain %q\n%s", expected, rendered)
		}
	}
	if strings.Contains(rendered, `local-data-ptr: "192.168.1.1`) {
		t.Fatal("PTR record was rendered for a host that did not request it")
	}
	if strings.Index(rendered, `local-zone: "home.lab." static`) > strings.Index(rendered, `local-data: "printer`) {
		t.Fatal("local-zone must be declared before its local-data entries")
	}
}

func TestLocalZoneValidationRejectsUnsafeInputs(t *testing.T) {
	tests := []struct {
		name string
		zone LocalZone
	}{
		{name: "non canonical zone name", zone: LocalZone{Name: "Home.Lab", Hosts: []LocalHost{{Hostname: "printer", IPv4: "192.168.1.20"}}}},
		{name: "no hosts", zone: LocalZone{Name: "home.lab.", Hosts: []LocalHost{}}},
		{name: "uppercase hostname", zone: LocalZone{Name: "home.lab.", Hosts: []LocalHost{{Hostname: "Printer", IPv4: "192.168.1.20"}}}},
		{name: "hostname with dot", zone: LocalZone{Name: "home.lab.", Hosts: []LocalHost{{Hostname: "printer.floor1", IPv4: "192.168.1.20"}}}},
		{name: "duplicate hostname", zone: LocalZone{Name: "home.lab.", Hosts: []LocalHost{
			{Hostname: "printer", IPv4: "192.168.1.20"},
			{Hostname: "printer", IPv4: "192.168.1.21"},
		}}},
		{name: "no address", zone: LocalZone{Name: "home.lab.", Hosts: []LocalHost{{Hostname: "printer"}}}},
		{name: "non canonical IPv4", zone: LocalZone{Name: "home.lab.", Hosts: []LocalHost{{Hostname: "printer", IPv4: "192.168.001.20"}}}},
		{name: "IPv6 in IPv4 field", zone: LocalZone{Name: "home.lab.", Hosts: []LocalHost{{Hostname: "printer", IPv4: "2001:db8::20"}}}},
		{name: "IPv4 in IPv6 field", zone: LocalZone{Name: "home.lab.", Hosts: []LocalHost{{Hostname: "printer", IPv6: "192.168.1.20"}}}},
		{name: "loopback address", zone: LocalZone{Name: "home.lab.", Hosts: []LocalHost{{Hostname: "printer", IPv4: "127.0.0.1"}}}},
		{name: "unspecified address", zone: LocalZone{Name: "home.lab.", Hosts: []LocalHost{{Hostname: "printer", IPv4: "0.0.0.0"}}}},
		{name: "multicast address", zone: LocalZone{Name: "home.lab.", Hosts: []LocalHost{{Hostname: "printer", IPv4: "224.0.0.1"}}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			settings := DefaultSettings()
			settings.LocalZones = []LocalZone{test.zone}
			if err := settings.Validate(); !errors.Is(err, ErrInvalidSettings) {
				t.Fatalf("expected ErrInvalidSettings, got %v", err)
			}
		})
	}
}

func TestLocalZoneValidationRejectsDuplicateZonesAndLimits(t *testing.T) {
	settings := DefaultSettings()
	settings.LocalZones = []LocalZone{
		{Name: "home.lab.", Hosts: []LocalHost{{Hostname: "printer", IPv4: "192.168.1.20"}}},
		{Name: "home.lab.", Hosts: []LocalHost{{Hostname: "router", IPv4: "192.168.1.1"}}},
	}
	if err := settings.Validate(); !errors.Is(err, ErrInvalidSettings) {
		t.Fatalf("expected duplicate zone rejection, got %v", err)
	}

	settings = DefaultSettings()
	settings.LocalZones = make([]LocalZone, maxLocalZones+1)
	for index := range settings.LocalZones {
		settings.LocalZones[index] = LocalZone{
			Name:  zoneNameForIndex(index),
			Hosts: []LocalHost{{Hostname: "host", IPv4: "192.168.1.1"}},
		}
	}
	if err := settings.Validate(); !errors.Is(err, ErrInvalidSettings) {
		t.Fatalf("expected zone limit rejection, got %v", err)
	}

	settings = DefaultSettings()
	hosts := make([]LocalHost, maxLocalHostsPerZone+1)
	for index := range hosts {
		hosts[index] = LocalHost{Hostname: hostNameForIndex(index), IPv4: "192.168.1.1"}
	}
	settings.LocalZones = []LocalZone{{Name: "home.lab.", Hosts: hosts}}
	if err := settings.Validate(); !errors.Is(err, ErrInvalidSettings) {
		t.Fatalf("expected per-zone host limit rejection, got %v", err)
	}

	perZoneHosts := make([]LocalHost, maxLocalHostsPerZone)
	for index := range perZoneHosts {
		perZoneHosts[index] = LocalHost{Hostname: hostNameForIndex(index), IPv4: "192.168.1.1"}
	}
	zonesNeeded := maxLocalHostsTotal/maxLocalHostsPerZone + 1
	settings = DefaultSettings()
	settings.LocalZones = make([]LocalZone, zonesNeeded)
	for index := range settings.LocalZones {
		settings.LocalZones[index] = LocalZone{Name: zoneNameForIndex(index), Hosts: perZoneHosts}
	}
	if err := settings.Validate(); !errors.Is(err, ErrInvalidSettings) {
		t.Fatalf("expected total host limit rejection, got %v", err)
	}
}

func TestLocalZonePTRRejectsDuplicateAddressAcrossZones(t *testing.T) {
	settings := DefaultSettings()
	settings.LocalZones = []LocalZone{
		{Name: "home.lab.", Hosts: []LocalHost{{Hostname: "printer", IPv4: "192.168.1.20", PTR: true}}},
		{Name: "guests.lab.", Hosts: []LocalHost{{Hostname: "printer", IPv4: "192.168.1.20", PTR: true}}},
	}
	if err := settings.Validate(); !errors.Is(err, ErrInvalidSettings) {
		t.Fatalf("expected PTR address conflict rejection, got %v", err)
	}

	// The same address without PTR on both sides is not a conflict - only one
	// PTR record can exist per address, but plain forward records can repeat.
	settings.LocalZones[0].Hosts[0].PTR = false
	settings.LocalZones[1].Hosts[0].PTR = false
	if err := settings.Validate(); err != nil {
		t.Fatalf("expected non-PTR duplicate addresses to be allowed, got %v", err)
	}
}

func zoneNameForIndex(index int) string {
	return "zone" + strconv.Itoa(index) + ".lab."
}

func hostNameForIndex(index int) string {
	return "host" + strconv.Itoa(index)
}
