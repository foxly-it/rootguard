package routerimport

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	maxReverseDNSAddresses = 256
	reverseDNSWorkers      = 16
	reverseDNSBudget       = 15 * time.Second
	sourceReverseDNS       = "reverse-dns"
)

var ErrReverseDNSDiscovery = errors.New("reverse DNS discovery failed")

type reverseResolver interface {
	LookupAddr(ctx context.Context, address string) ([]string, error)
}

type ReverseDNSDiscoverer struct {
	resolver reverseResolver
}

func NewReverseDNSDiscoverer() *ReverseDNSDiscoverer {
	return &ReverseDNSDiscoverer{resolver: net.DefaultResolver}
}

func (d *ReverseDNSDiscoverer) Discover(ctx context.Context, networks []string) (DiscoveryResult, error) {
	addresses, err := reverseDNSAddresses(networks)
	if err != nil {
		return DiscoveryResult{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, reverseDNSBudget)
	defer cancel()

	jobs := make(chan netip.Addr)
	var hosts []DiscoveredHost
	failed := 0
	var mu sync.Mutex
	var workers sync.WaitGroup
	for range min(reverseDNSWorkers, len(addresses)) {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for address := range jobs {
				names, lookupErr := d.resolver.LookupAddr(ctx, address.String())
				mu.Lock()
				if lookupErr != nil {
					var dnsErr *net.DNSError
					if !errors.As(lookupErr, &dnsErr) || !dnsErr.IsNotFound {
						failed++
					}
				} else if name := firstPTRName(names); name != "" {
					host := DiscoveredHost{Hostname: name, Active: true, Source: sourceReverseDNS}
					if address.Is4() {
						host.IPv4 = address.String()
					} else {
						host.IPv6 = address.String()
					}
					hosts = append(hosts, host)
				}
				mu.Unlock()
			}
		}()
	}
	for _, address := range addresses {
		select {
		case jobs <- address:
		case <-ctx.Done():
			close(jobs)
			workers.Wait()
			return DiscoveryResult{}, fmt.Errorf("%w: %v", ErrReverseDNSDiscovery, ctx.Err())
		}
	}
	close(jobs)
	workers.Wait()
	if err := ctx.Err(); err != nil {
		return DiscoveryResult{}, fmt.Errorf("%w: %v", ErrReverseDNSDiscovery, err)
	}

	sort.Slice(hosts, func(i, j int) bool {
		left, _ := hostAddress(hosts[i])
		right, _ := hostAddress(hosts[j])

		return left.Less(right)
	})

	return DiscoveryResult{Hosts: hosts, Scanned: len(addresses), Failed: failed}, nil
}

func reverseDNSAddresses(networks []string) ([]netip.Addr, error) {
	if len(networks) == 0 {
		return nil, fmt.Errorf("%w: at least one network is required", ErrReverseDNSDiscovery)
	}
	seen := make(map[netip.Addr]struct{})
	for _, raw := range networks {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(raw))
		if err != nil || prefix != prefix.Masked() {
			return nil, fmt.Errorf("%w: %q is not a canonical CIDR prefix", ErrReverseDNSDiscovery, raw)
		}
		address := prefix.Addr()
		if address.Is4() && !address.IsPrivate() {
			return nil, fmt.Errorf("%w: IPv4 network %q is not private", ErrReverseDNSDiscovery, raw)
		}
		if !address.Is4() && (!address.IsGlobalUnicast() || address.IsLinkLocalUnicast()) {
			return nil, fmt.Errorf("%w: IPv6 network %q is not unicast", ErrReverseDNSDiscovery, raw)
		}
		if prefix.Addr().BitLen()-prefix.Bits() > 8 {
			return nil, fmt.Errorf("%w: network %q contains more than %d addresses", ErrReverseDNSDiscovery, raw, maxReverseDNSAddresses)
		}
		for current := prefix.Addr(); prefix.Contains(current); current = current.Next() {
			seen[current] = struct{}{}
			if len(seen) > maxReverseDNSAddresses {
				return nil, fmt.Errorf("%w: networks contain more than %d addresses", ErrReverseDNSDiscovery, maxReverseDNSAddresses)
			}
		}
	}
	addresses := make([]netip.Addr, 0, len(seen))
	for address := range seen {
		addresses = append(addresses, address)
	}
	sort.Slice(addresses, func(i, j int) bool { return addresses[i].Less(addresses[j]) })

	return addresses, nil
}

func firstPTRName(names []string) string {
	for i := range names {
		names[i] = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(names[i])), ".")
	}
	sort.Strings(names)
	for _, name := range names {
		if name != "" {
			return name
		}
	}

	return ""
}

func hostAddress(host DiscoveredHost) (netip.Addr, error) {
	if host.IPv4 != "" {
		return netip.ParseAddr(host.IPv4)
	}

	return netip.ParseAddr(host.IPv6)
}
