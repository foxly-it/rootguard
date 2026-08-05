package unbound

import (
	"context"
	"strings"
	"time"
)

type NetworkCapabilities struct {
	IPv4Available bool      `json:"ipv4_available"`
	IPv4Detail    string    `json:"ipv4_detail"`
	IPv6Available bool      `json:"ipv6_available"`
	IPv6Detail    string    `json:"ipv6_detail"`
	CheckedAt     time.Time `json:"checked_at"`
}

func (m *Manager) NetworkCapabilities(ctx context.Context) NetworkCapabilities {
	probe := func(family, server string) (bool, string) {
		probeCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
		defer cancel()
		output, err := m.run(probeCtx, "docker", "exec", m.containerName,
			"dig", family, "+time=2", "+tries=1", "+short", "@"+server, ".", "NS")
		detail := strings.TrimSpace(string(output))
		if err != nil || detail == "" {
			if detail == "" {
				detail = "No authoritative root-server response was received."
			}
			return false, detail
		}
		return true, "Authoritative DNS connectivity is available."
	}
	ipv4, ipv4Detail := probe("-4", "198.41.0.4")
	ipv6, ipv6Detail := probe("-6", "2001:503:ba3e::2:30")
	return NetworkCapabilities{
		IPv4Available: ipv4,
		IPv4Detail:    ipv4Detail,
		IPv6Available: ipv6,
		IPv6Detail:    ipv6Detail,
		CheckedAt:     m.now().UTC(),
	}
}
