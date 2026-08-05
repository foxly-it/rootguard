package unbound

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	forwardCheckWorkers       = 4
	forwardCheckTargetTimeout = 4 * time.Second
	forwardCheckTotalTimeout  = 20 * time.Second
	maxForwardCheckDetail     = 512
)

type ForwardTargetCheck struct {
	Zone      string `json:"zone"`
	Address   string `json:"address"`
	Reachable bool   `json:"reachable"`
	Detail    string `json:"detail"`
}

func (m *Manager) CheckForwardTargets(ctx context.Context, zones []ForwardZone) ([]ForwardTargetCheck, error) {
	settings := DefaultSettings()
	settings.ForwardZones = zones
	if err := settings.Validate(); err != nil {
		return nil, err
	}

	type target struct {
		index   int
		zone    string
		address string
	}
	targets := make([]target, 0)
	for _, zone := range zones {
		for _, address := range zone.Servers {
			targets = append(targets, target{index: len(targets), zone: zone.Name, address: address})
		}
	}
	results := make([]ForwardTargetCheck, len(targets))
	if len(targets) == 0 {
		return results, nil
	}

	ctx, cancel := context.WithTimeout(ctx, forwardCheckTotalTimeout)
	defer cancel()
	jobs := make(chan target)
	var workers sync.WaitGroup
	workerCount := min(forwardCheckWorkers, len(targets))
	workers.Add(workerCount)
	for range workerCount {
		go func() {
			defer workers.Done()
			for item := range jobs {
				results[item.index] = m.checkForwardTarget(ctx, item.zone, item.address)
			}
		}()
	}
	for _, item := range targets {
		select {
		case jobs <- item:
		case <-ctx.Done():
			close(jobs)
			workers.Wait()
			return results, fmt.Errorf("forward target checks timed out: %w", ctx.Err())
		}
	}
	close(jobs)
	workers.Wait()
	return results, nil
}

func (m *Manager) checkForwardTarget(ctx context.Context, zone, address string) ForwardTargetCheck {
	targetContext, cancel := context.WithTimeout(ctx, forwardCheckTargetTimeout)
	defer cancel()
	output, err := m.run(
		targetContext,
		"docker", "exec", m.containerName,
		"dig", "+time=3", "+tries=1", "+noall", "+comments", "+answer", "+authority", "@"+address, zone, "SOA",
	)
	detail := strings.TrimSpace(string(output))
	if err != nil {
		if detail == "" {
			detail = err.Error()
		} else {
			detail = fmt.Sprintf("%v: %s", err, detail)
		}
		return ForwardTargetCheck{Zone: zone, Address: address, Detail: boundForwardCheckDetail(detail)}
	}
	status := dnsResponseStatus(detail)
	if status != "NOERROR" {
		return ForwardTargetCheck{
			Zone: zone, Address: address,
			Detail: rejectedForwardResponseDetail(status),
		}
	}
	hasSOA := hasSOARecord(detail)
	if !hasSOA {
		detail = rejectedForwardResponseDetail(status)
	}
	return ForwardTargetCheck{
		Zone: zone, Address: address,
		Reachable: hasSOA,
		Detail:    boundForwardCheckDetail(detail),
	}
}

func dnsResponseStatus(detail string) string {
	const marker = "status:"
	index := strings.Index(detail, marker)
	if index < 0 {
		return ""
	}
	fields := strings.Fields(detail[index+len(marker):])
	if len(fields) == 0 {
		return ""
	}
	return strings.TrimSuffix(fields[0], ",")
}

func rejectedForwardResponseDetail(status string) string {
	if status == "" {
		return "DNS server did not return an SOA record for the forwarding zone (missing response status)"
	}
	return fmt.Sprintf("DNS server did not return an SOA record for the forwarding zone (status: %s)", status)
}

func hasSOARecord(detail string) bool {
	for line := range strings.SplitSeq(detail, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, ";") {
			continue
		}
		for _, field := range strings.Fields(line) {
			if field == "SOA" {
				return true
			}
		}
	}
	return false
}

func boundForwardCheckDetail(detail string) string {
	if len(detail) > maxForwardCheckDetail {
		return detail[:maxForwardCheckDetail] + "…"
	}
	return detail
}
