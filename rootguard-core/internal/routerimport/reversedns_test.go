package routerimport

import (
	"context"
	"errors"
	"net"
	"reflect"
	"testing"
)

type reverseResolverFunc func(ctx context.Context, address string) ([]string, error)

func (f reverseResolverFunc) LookupAddr(ctx context.Context, address string) ([]string, error) {
	return f(ctx, address)
}

func TestReverseDNSDiscoverer(t *testing.T) {
	records := map[string][]string{
		"192.168.50.1": {"router.home.arpa."},
		"192.168.50.2": {"printer.home.arpa.", "printer.local."},
		"fd00::1":      {"server.home.arpa."},
	}
	discoverer := &ReverseDNSDiscoverer{resolver: reverseResolverFunc(func(_ context.Context, address string) ([]string, error) {
		if names, ok := records[address]; ok {
			return names, nil
		}

		return nil, &net.DNSError{IsNotFound: true}
	})}

	result, err := discoverer.Discover(context.Background(), []string{"192.168.50.0/30", "fd00::/127"})
	if err != nil {
		t.Fatalf("discover reverse DNS: %v", err)
	}
	if result.Scanned != 6 || result.Failed != 0 {
		t.Fatalf("unexpected discovery counters: %+v", result)
	}
	want := []DiscoveredHost{
		{Hostname: "router.home.arpa", IPv4: "192.168.50.1", Active: true, Source: sourceReverseDNS},
		{Hostname: "printer.home.arpa", IPv4: "192.168.50.2", Active: true, Source: sourceReverseDNS},
		{Hostname: "server.home.arpa", IPv6: "fd00::1", Active: true, Source: sourceReverseDNS},
	}
	if !reflect.DeepEqual(result.Hosts, want) {
		t.Fatalf("unexpected hosts:\n got: %+v\nwant: %+v", result.Hosts, want)
	}
}

func TestReverseDNSDiscovererCountsResolverFailures(t *testing.T) {
	discoverer := &ReverseDNSDiscoverer{resolver: reverseResolverFunc(func(_ context.Context, _ string) ([]string, error) {
		return nil, errors.New("resolver unavailable")
	})}
	result, err := discoverer.Discover(context.Background(), []string{"192.168.1.1/32"})
	if err != nil {
		t.Fatalf("discover reverse DNS: %v", err)
	}
	if result.Scanned != 1 || result.Failed != 1 || len(result.Hosts) != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.Hosts == nil {
		t.Fatal("expected an empty host list, got nil")
	}
}

func TestReverseDNSDiscovererRejectsUnsafeRanges(t *testing.T) {
	tests := []string{
		"",
		"192.168.1.1/24",
		"192.168.0.0/23",
		"203.0.113.0/24",
		"fd00::/119",
		"fe80::/120",
	}
	for _, network := range tests {
		t.Run(network, func(t *testing.T) {
			_, err := reverseDNSAddresses([]string{network})
			if !errors.Is(err, ErrReverseDNSDiscovery) {
				t.Fatalf("expected ErrReverseDNSDiscovery, got %v", err)
			}
		})
	}
}

func TestReverseDNSAddressesDeduplicatesOverlaps(t *testing.T) {
	addresses, err := reverseDNSAddresses([]string{"10.0.0.0/31", "10.0.0.1/32"})
	if err != nil {
		t.Fatalf("enumerate addresses: %v", err)
	}
	if len(addresses) != 2 {
		t.Fatalf("expected two unique addresses, got %d", len(addresses))
	}
}
