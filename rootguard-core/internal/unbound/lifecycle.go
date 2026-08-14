package unbound

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const historyLimit = 20

var ErrVersionNotFound = errors.New("unbound configuration version not found")

type Change struct {
	Field  string `json:"field"`
	Before string `json:"before"`
	After  string `json:"after"`
}

type Preview struct {
	Changed        bool     `json:"changed"`
	Changes        []Change `json:"changes"`
	RenderedConfig string   `json:"rendered_config"`
}

type HistoryEntry struct {
	ID           string    `json:"id"`
	CreatedAt    time.Time `json:"created_at"`
	Settings     Settings  `json:"settings"`
	Config       string    `json:"config,omitempty"`
	CustomConfig string    `json:"custom_config,omitempty"`
}

type DiagnosticCheck struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail"`
}

type DiagnosticReport struct {
	Healthy   bool              `json:"healthy"`
	CheckedAt time.Time         `json:"checked_at"`
	Checks    []DiagnosticCheck `json:"checks"`
}

func (m *Manager) Preview(settings Settings) (Preview, error) {
	config, err := settings.Render()
	if err != nil {
		return Preview{}, err
	}
	custom, err := m.LoadCustom()
	if err != nil {
		return Preview{}, err
	}
	if err := validateGuidedConflicts(settings, custom.Content); err != nil {
		return Preview{}, err
	}
	current, err := m.Load()
	if err != nil {
		return Preview{}, err
	}
	changes := settingsChanges(current, settings)
	return Preview{Changed: len(changes) > 0, Changes: changes, RenderedConfig: string(config)}, nil
}

func settingsChanges(before, after Settings) []Change {
	changes := make([]Change, 0, 10)
	add := func(field string, oldValue, newValue any) {
		oldText, newText := fmt.Sprint(oldValue), fmt.Sprint(newValue)
		if oldText != newText {
			changes = append(changes, Change{Field: field, Before: oldText, After: newText})
		}
	}
	add("qname_minimisation", before.QnameMinimisation, after.QnameMinimisation)
	add("prefetch", before.Prefetch, after.Prefetch)
	add("prefetch_key", before.PrefetchKey, after.PrefetchKey)
	add("aggressive_nsec", before.AggressiveNSEC, after.AggressiveNSEC)
	add("edns_buffer_size", before.EDNSBufferSize, after.EDNSBufferSize)
	add("log_verbosity", before.LogVerbosity, after.LogVerbosity)
	add("serve_expired", before.ServeExpired, after.ServeExpired)
	add("serve_expired_ttl", before.ServeExpiredTTL, after.ServeExpiredTTL)
	add("serve_expired_client_timeout", before.ServeExpiredClientTimeout, after.ServeExpiredClientTimeout)
	add("cache_min_ttl", before.CacheMinTTL, after.CacheMinTTL)
	add("cache_max_ttl", before.CacheMaxTTL, after.CacheMaxTTL)
	add("threads", before.Threads, after.Threads)
	add("resource_profile", before.ResourceProfile, after.ResourceProfile)
	add("network_mode", before.NetworkMode, after.NetworkMode)
	if !forwardZonesEqual(before.ForwardZones, after.ForwardZones) {
		changes = append(changes, Change{
			Field:  "forward_zones",
			Before: formatForwardZones(before.ForwardZones),
			After:  formatForwardZones(after.ForwardZones),
		})
	}
	if !slicesEqual(before.PrivateDomains, after.PrivateDomains) {
		changes = append(changes, Change{
			Field: "private_domains", Before: formatJSON(before.PrivateDomains),
			After: formatJSON(after.PrivateDomains),
		})
	}
	if !slicesEqual(before.ReverseZones, after.ReverseZones) {
		changes = append(changes, Change{
			Field: "reverse_zones", Before: formatJSON(before.ReverseZones),
			After: formatJSON(after.ReverseZones),
		})
	}
	if !localZonesEqual(before.LocalZones, after.LocalZones) {
		changes = append(changes, Change{
			Field: "local_zones", Before: formatJSON(before.LocalZones),
			After: formatJSON(after.LocalZones),
		})
	}
	return changes
}

func slicesEqual[T comparable](left, right []T) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func forwardZonesEqual(left, right []ForwardZone) bool {
	return settingsEqual(
		Settings{ForwardZones: left},
		Settings{ForwardZones: right},
	)
}

func formatForwardZones(zones []ForwardZone) string {
	if len(zones) == 0 {
		return "[]"
	}
	data, err := json.Marshal(zones)
	if err != nil {
		return fmt.Sprintf("%d zones", len(zones))
	}
	return string(data)
}

func formatJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprint(value)
	}
	return string(data)
}

func (m *Manager) History() ([]HistoryEntry, error) {
	directory := filepath.Join(m.hostConfigDir, "history")
	files, err := os.ReadDir(directory)
	if errors.Is(err, os.ErrNotExist) {
		return []HistoryEntry{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read unbound history: %w", err)
	}
	entries := make([]HistoryEntry, 0, len(files))
	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(directory, file.Name()))
		if err != nil {
			return nil, err
		}
		var entry HistoryEntry
		if err := json.Unmarshal(data, &entry); err != nil {
			return nil, fmt.Errorf("decode unbound history %s: %w", file.Name(), err)
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].CreatedAt.After(entries[j].CreatedAt) })
	return entries, nil
}

func (m *Manager) Restore(ctx context.Context, id string) (Settings, error) {
	if id == "" || filepath.Base(id) != id || strings.ContainsAny(id, `/\\`) {
		return Settings{}, ErrVersionNotFound
	}
	data, err := os.ReadFile(filepath.Join(m.hostConfigDir, "history", id+".json"))
	if errors.Is(err, os.ErrNotExist) {
		return Settings{}, ErrVersionNotFound
	}
	if err != nil {
		return Settings{}, err
	}
	var entry HistoryEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return Settings{}, fmt.Errorf("decode unbound version: %w", err)
	}
	m.applyMu.Lock()
	defer m.applyMu.Unlock()
	if err := m.applyStateLocked(ctx, entry.Settings, entry.CustomConfig); err != nil {
		return Settings{}, err
	}
	return entry.Settings, nil
}

func (m *Manager) Diagnose(ctx context.Context) DiagnosticReport {
	checks := []DiagnosticCheck{
		m.diagnosticCommand(ctx, "configuration", "unbound-checkconf", "/etc/unbound/unbound.conf"),
		m.diagnosticResolution(ctx),
		m.diagnosticDNSSEC(ctx),
	}
	healthy := true
	for _, check := range checks {
		healthy = healthy && check.Passed
	}
	return DiagnosticReport{Healthy: healthy, CheckedAt: m.now().UTC(), Checks: checks}
}

func (m *Manager) diagnosticCommand(ctx context.Context, name string, args ...string) DiagnosticCheck {
	dockerArgs := append([]string{"exec", m.containerName}, args...)
	output, err := m.run(ctx, "docker", dockerArgs...)
	detail := strings.TrimSpace(string(output))
	if detail == "" {
		detail = "OK"
	}
	if err != nil {
		detail = fmt.Sprintf("%v: %s", err, detail)
	}
	return DiagnosticCheck{Name: name, Passed: err == nil, Detail: detail}
}

const digTimeout = "+time=5"

// resolutionCheck and dnssecCheck hold the shared dig-and-interpret logic
// behind diagnosticResolution/diagnosticPathResolution and
// diagnosticDNSSEC/diagnosticPathDNSSEC respectively - the pairs differ only
// in which host/port they query, the retry count, and the fallback detail
// text on failure.
func (m *Manager) resolutionCheck(ctx context.Context, name, host, port, tries, noAddressDetail string) DiagnosticCheck {
	check := m.diagnosticCommand(ctx, name, "dig", "+short", digTimeout, tries, "@"+host, "-p", port, "example.com", "A")
	check.Passed = check.Passed && strings.TrimSpace(check.Detail) != "" && check.Detail != "OK"
	if !check.Passed && check.Detail == "OK" {
		check.Detail = noAddressDetail
	}
	return check
}

func (m *Manager) dnssecCheck(ctx context.Context, name, host, port, tries, rejectedDetailPrefix string) DiagnosticCheck {
	check := m.diagnosticCommand(ctx, name, "dig", "+dnssec", digTimeout, tries, "@"+host, "-p", port, "dnssec-failed.org", "A")
	check.Passed = check.Passed && strings.Contains(check.Detail, "status: SERVFAIL")
	if !check.Passed && !strings.Contains(check.Detail, "SERVFAIL") {
		check.Detail = rejectedDetailPrefix + check.Detail
	}
	return check
}

func (m *Manager) diagnosticResolution(ctx context.Context) DiagnosticCheck {
	return m.resolutionCheck(ctx, "resolution", "127.0.0.1", "5335", "+tries=1", "resolver returned no address")
}

func (m *Manager) diagnosticDNSSEC(ctx context.Context) DiagnosticCheck {
	return m.dnssecCheck(ctx, "dnssec", "127.0.0.1", "5335", "+tries=1", "invalid DNSSEC response was not rejected: ")
}

// DiagnosePath verifies the DNS path a real client actually uses - through
// AdGuard's own listener, not Unbound's - resolves and rejects invalid
// DNSSEC the same way Diagnose verifies Unbound's own port in isolation.
// Both checks still run as dig inside rootguard-unbound: it's the only
// container in the stack with both DNS tooling and network line-of-sight to
// AdGuard, and Core itself has no DNS client of its own (see adguard.Manager,
// which only ever speaks AdGuard's HTTP control API). Querying AdGuard's
// container-internal port 53 directly, not the host-published bind address,
// keeps this a control-plane health check independent of whatever DNS port
// the operator configured for their LAN.
func (m *Manager) DiagnosePath(ctx context.Context, adguardAddress string) DiagnosticReport {
	checks := []DiagnosticCheck{
		m.diagnosticPathResolution(ctx, adguardAddress),
		m.diagnosticPathDNSSEC(ctx, adguardAddress),
	}
	healthy := true
	for _, check := range checks {
		healthy = healthy && check.Passed
	}
	return DiagnosticReport{Healthy: healthy, CheckedAt: m.now().UTC(), Checks: checks}
}

func (m *Manager) diagnosticPathResolution(ctx context.Context, adguardAddress string) DiagnosticCheck {
	host, port, err := net.SplitHostPort(adguardAddress)
	if err != nil {
		return DiagnosticCheck{Name: "adguard-resolution", Passed: false, Detail: fmt.Sprintf("invalid AdGuard DNS address %q: %v", adguardAddress, err)}
	}
	return m.resolutionCheck(ctx, "adguard-resolution", host, port, "+tries=2", "AdGuard returned no address")
}

func (m *Manager) diagnosticPathDNSSEC(ctx context.Context, adguardAddress string) DiagnosticCheck {
	host, port, err := net.SplitHostPort(adguardAddress)
	if err != nil {
		return DiagnosticCheck{Name: "adguard-dnssec", Passed: false, Detail: fmt.Sprintf("invalid AdGuard DNS address %q: %v", adguardAddress, err)}
	}
	return m.dnssecCheck(ctx, "adguard-dnssec", host, port, "+tries=2", "invalid DNSSEC response was not rejected via AdGuard: ")
}

func (m *Manager) recordSnapshot(settings Settings, config, custom []byte) error {
	history, err := m.History()
	if err != nil {
		return err
	}
	if len(history) > 0 && settingsEqual(history[0].Settings, settings) && history[0].Config == string(config) && history[0].CustomConfig == string(custom) {
		return nil
	}
	digest := sha256.Sum256(append(append(bytes.Clone(config), 0), custom...))
	createdAt := m.now().UTC()
	id := createdAt.Format("20060102T150405.000000000Z") + "-" + hex.EncodeToString(digest[:4])
	entry := HistoryEntry{ID: id, CreatedAt: createdAt, Settings: settings, Config: string(config), CustomConfig: string(custom)}
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return err
	}
	directory := filepath.Join(m.hostConfigDir, "history")
	if err := os.MkdirAll(directory, 0700); err != nil {
		return err
	}
	if err := writeAtomic(filepath.Join(directory, id+".json"), append(data, '\n'), 0600); err != nil {
		return err
	}
	return m.pruneHistory(directory)
}

func (m *Manager) pruneHistory(directory string) error {
	history, err := m.History()
	if err != nil {
		return err
	}
	if len(history) <= historyLimit {
		return nil
	}
	for _, entry := range history[historyLimit:] {
		if err := os.Remove(filepath.Join(directory, entry.ID+".json")); err != nil {
			return err
		}
	}
	return nil
}
