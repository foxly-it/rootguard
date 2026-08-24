package adguard

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type Status struct {
	Configured         bool    `json:"configured"`
	Healthy            bool    `json:"healthy"`
	Version            string  `json:"version,omitempty"`
	Upstream           string  `json:"upstream"`
	UpstreamReady      bool    `json:"upstream_ready"`
	StatsAvailable     bool    `json:"stats_available"`
	Queries            uint64  `json:"queries"`
	Blocked            uint64  `json:"blocked"`
	AverageResponse    float64 `json:"average_response_seconds"`
	BestPracticesReady bool    `json:"best_practices_ready"`
	FilteringEnabled   bool    `json:"filtering_enabled"`
	ActiveFilterLists  int     `json:"active_filter_lists"`
	TotalFilterLists   int     `json:"total_filter_lists"`
	// ProtectionEnabled mirrors AdGuard Home's own "Protection" switch (the
	// dashboard dropdown offering Off/10 minutes/1 hour) - distinct from
	// FilteringEnabled above, which only controls whether the blocklists are
	// applied. See SetProtection.
	ProtectionEnabled bool `json:"protection_enabled"`
}

type FilterCheck struct {
	Host            string `json:"host"`
	Category        string `json:"category"`
	ExpectedBlocked bool   `json:"expected_blocked"`
	Blocked         bool   `json:"blocked"`
	Reason          string `json:"reason"`
	MatchedRule     string `json:"matched_rule,omitempty"`
}

type FilterReport struct {
	Checks    []FilterCheck `json:"checks"`
	Blocked   int           `json:"blocked"`
	Expected  int           `json:"expected"`
	Passed    int           `json:"passed"`
	CheckedAt time.Time     `json:"checked_at"`
}

type filterProbe struct {
	host            string
	category        string
	expectedBlocked bool
}

var filterProbes = []filterProbe{
	{host: "googlesyndication.com", category: "advertising", expectedBlocked: true},
	{host: "doubleverify.com", category: "tracking", expectedBlocked: true},
	{host: "bid.g.doubleclick.net", category: "advertising", expectedBlocked: true},
	{host: "amazon-adsystem.com", category: "advertising", expectedBlocked: true},
	{host: "tiktok.com", category: "service", expectedBlocked: false},
	{host: "connect.facebook.net", category: "tracking", expectedBlocked: true},
	{host: "mc.yandex.ru", category: "tracking", expectedBlocked: true},
	{host: "samsungcloudplatform.com", category: "telemetry", expectedBlocked: false},
	{host: "microsoft.com", category: "service", expectedBlocked: false},
	{host: "amtso.org", category: "security-test", expectedBlocked: false},
	{host: "wicar.org", category: "security-test", expectedBlocked: false},
}

type Manager struct {
	installerURL     string
	apiURL           string
	dataDir          string
	upstream         string
	blockpageAuthDir string
	http             *http.Client
	retryDelay       time.Duration
}

func NewManager(installerURL, apiURL, dataDir, upstream, blockpageAuthDir string) *Manager {
	return &Manager{
		installerURL:     strings.TrimRight(installerURL, "/"),
		apiURL:           strings.TrimRight(apiURL, "/"),
		dataDir:          dataDir,
		upstream:         upstream,
		blockpageAuthDir: blockpageAuthDir,
		http:             &http.Client{Timeout: 10 * time.Second},
		retryDelay:       500 * time.Millisecond,
	}
}

func (m *Manager) Status(ctx context.Context) (Status, error) {
	credentials, err := m.loadCredentials()
	if errors.Is(err, os.ErrNotExist) {
		if err := m.request(ctx, http.MethodGet, m.installerURL+"/control/install/get_addresses", nil, nil, nil); err != nil {
			return Status{}, fmt.Errorf("adguard is neither configured nor reachable through its installer: %w", err)
		}
		return Status{Upstream: m.upstream}, nil
	}
	if err != nil {
		return Status{}, err
	}

	var controlStatus struct {
		Version string `json:"version"`
	}
	if err := m.request(ctx, http.MethodGet, m.apiURL+"/control/status", nil, &controlStatus, &credentials); err != nil {
		return Status{}, fmt.Errorf("adguard status: %w", err)
	}

	var dnsInfo struct {
		UpstreamDNS        []string `json:"upstream_dns"`
		FallbackDNS        []string `json:"fallback_dns"`
		ProtectionEnabled  bool     `json:"protection_enabled"`
		RateLimit          int      `json:"ratelimit"`
		DNSSECEnabled      bool     `json:"dnssec_enabled"`
		EDNSCSEnabled      bool     `json:"edns_cs_enabled"`
		CacheEnabled       bool     `json:"cache_enabled"`
		CacheSize          int      `json:"cache_size"`
		CacheOptimistic    bool     `json:"cache_optimistic"`
		BlockedResponseTTL int      `json:"blocked_response_ttl"`
		UpstreamTimeout    int      `json:"upstream_timeout"`
	}
	if err := m.request(ctx, http.MethodGet, m.apiURL+"/control/dns_info", nil, &dnsInfo, &credentials); err != nil {
		return Status{}, fmt.Errorf("adguard dns info: %w", err)
	}

	status := Status{
		Configured: true,
		Healthy:    true,
		Version:    controlStatus.Version,
		Upstream:   m.upstream,
		UpstreamReady: len(dnsInfo.UpstreamDNS) == 1 &&
			dnsInfo.UpstreamDNS[0] == m.upstream && len(dnsInfo.FallbackDNS) == 0,
		ProtectionEnabled: dnsInfo.ProtectionEnabled,
		BestPracticesReady: dnsInfo.ProtectionEnabled &&
			dnsInfo.RateLimit == 20 &&
			dnsInfo.DNSSECEnabled &&
			!dnsInfo.EDNSCSEnabled &&
			dnsInfo.CacheEnabled &&
			dnsInfo.CacheSize >= 4*1024*1024 &&
			!dnsInfo.CacheOptimistic &&
			dnsInfo.BlockedResponseTTL == 10 &&
			dnsInfo.UpstreamTimeout == 10,
	}
	var stats struct {
		Queries         uint64  `json:"num_dns_queries"`
		Blocked         uint64  `json:"num_blocked_filtering"`
		AverageResponse float64 `json:"avg_processing_time"`
	}
	if err := m.request(ctx, http.MethodGet, m.apiURL+"/control/stats", nil, &stats, &credentials); err == nil {
		status.StatsAvailable = true
		status.Queries = stats.Queries
		status.Blocked = stats.Blocked
		status.AverageResponse = stats.AverageResponse
	}
	if filtering, err := m.filteringStatus(ctx, &credentials); err == nil {
		status.FilteringEnabled = filtering.Enabled
		status.TotalFilterLists = len(filtering.Filters)
		for _, list := range filtering.Filters {
			if list.Enabled {
				status.ActiveFilterLists++
			}
		}
	}
	return status, nil
}

type filteringStatusResponse struct {
	Enabled  bool `json:"enabled"`
	Interval int  `json:"interval"`
	Filters  []struct {
		Enabled bool `json:"enabled"`
	} `json:"filters"`
}

func (m *Manager) filteringStatus(ctx context.Context, credentials *Credentials) (filteringStatusResponse, error) {
	var response filteringStatusResponse
	if err := m.request(ctx, http.MethodGet, m.apiURL+"/control/filtering/status", nil, &response, credentials); err != nil {
		return filteringStatusResponse{}, fmt.Errorf("adguard filtering status: %w", err)
	}
	return response, nil
}

// SetFiltering toggles AdGuard's master filtering switch. The interval is
// read back from AdGuard first and resubmitted unchanged - the endpoint
// requires both fields together, and this call has no business changing
// the update interval the operator configured natively.
func (m *Manager) SetFiltering(ctx context.Context, enabled bool) (Status, error) {
	credentials, err := m.loadCredentials()
	if err != nil {
		return Status{}, err
	}
	current, err := m.filteringStatus(ctx, &credentials)
	if err != nil {
		return Status{}, err
	}
	body := struct {
		Enabled  bool `json:"enabled"`
		Interval int  `json:"interval"`
	}{Enabled: enabled, Interval: current.Interval}
	if err := m.request(ctx, http.MethodPost, m.apiURL+"/control/filtering/config", body, nil, &credentials); err != nil {
		return Status{}, fmt.Errorf("adguard set filtering: %w", err)
	}
	return m.Status(ctx)
}

// SetProtection mirrors AdGuard Home's own "Protection" dropdown (Off/10
// minutes/1 hour, shown at the top of its native dashboard) - unlike
// FilteringEnabled/SetFiltering, AdGuard has a real, server-side timer for
// this: passing a positive duration disables protection and has AdGuard
// itself re-enable it once that duration elapses, with no RootGuard-side
// scheduling required. duration <= 0 disables (or enables) with no timer.
func (m *Manager) SetProtection(ctx context.Context, enabled bool, duration time.Duration) (Status, error) {
	credentials, err := m.loadCredentials()
	if err != nil {
		return Status{}, err
	}
	body := struct {
		Enabled  bool  `json:"enabled"`
		Duration int64 `json:"duration"`
	}{Enabled: enabled}
	if duration > 0 {
		body.Duration = duration.Milliseconds()
	}
	if err := m.request(ctx, http.MethodPost, m.apiURL+"/control/protection", body, nil, &credentials); err != nil {
		return Status{}, fmt.Errorf("adguard set protection: %w", err)
	}
	return m.Status(ctx)
}

func (m *Manager) Bootstrap(ctx context.Context, blockPageIP string) (Status, error) {
	credentials, err := m.loadCredentials()
	if errors.Is(err, os.ErrNotExist) {
		credentials, err = m.install(ctx)
	}
	if err != nil {
		return Status{}, err
	}
	if err := m.waitUntilReady(ctx, credentials); err != nil {
		return Status{}, err
	}
	if err := m.configureUpstream(ctx, credentials, blockPageIP); err != nil {
		return Status{}, err
	}
	if blockPageIP != "" {
		if err := m.publishBlockpageAuthToken(credentials); err != nil {
			return Status{}, fmt.Errorf("publish blockpage auth token: %w", err)
		}
	}
	return m.Status(ctx)
}

// publishBlockpageAuthToken derives a Basic-Auth token from the AdGuard
// admin credentials and writes it to the volume shared with the blockpage
// container, so the blockpage's own nginx can proxy /api/reason to AdGuard's
// check_host endpoint without ever holding the raw username/password - the
// one component here that's reachable, unauthenticated, by anyone who gets
// DNS-sinkholed to it.
func (m *Manager) publishBlockpageAuthToken(credentials Credentials) error {
	if m.blockpageAuthDir == "" {
		return nil
	}
	if err := os.MkdirAll(m.blockpageAuthDir, 0700); err != nil {
		return fmt.Errorf("create blockpage auth directory: %w", err)
	}
	token := base64.StdEncoding.EncodeToString([]byte(credentials.Username + ":" + credentials.Password))
	tempPath := filepath.Join(m.blockpageAuthDir, ".basic-auth-token.tmp")
	if err := os.WriteFile(tempPath, []byte(token), 0600); err != nil {
		return fmt.Errorf("write blockpage auth token: %w", err)
	}
	defer os.Remove(tempPath)
	if err := os.Rename(tempPath, filepath.Join(m.blockpageAuthDir, "basic-auth-token")); err != nil {
		return fmt.Errorf("activate blockpage auth token: %w", err)
	}
	return nil
}

func (m *Manager) install(ctx context.Context) (Credentials, error) {
	if err := m.waitUntilInstallerReady(ctx); err != nil {
		return Credentials{}, err
	}

	credentials, err := generateCredentials()
	if err != nil {
		return Credentials{}, err
	}
	address := map[string]any{"ip": "0.0.0.0", "port": 80, "autofix": false}
	checkRequest := map[string]any{
		"web":           address,
		"dns":           map[string]any{"ip": "0.0.0.0", "port": 53, "autofix": false},
		"set_static_ip": false,
	}
	var checkResponse struct {
		Web struct {
			Status string `json:"status"`
		} `json:"web"`
		DNS struct {
			Status string `json:"status"`
		} `json:"dns"`
	}
	if err := m.request(ctx, http.MethodPost, m.installerURL+"/control/install/check_config", checkRequest, &checkResponse, nil); err != nil {
		return Credentials{}, fmt.Errorf("check adguard install config: %w", err)
	}
	if checkResponse.Web.Status != "" || checkResponse.DNS.Status != "" {
		return Credentials{}, fmt.Errorf("adguard rejected install addresses: web=%q dns=%q", checkResponse.Web.Status, checkResponse.DNS.Status)
	}

	if err := os.MkdirAll(m.dataDir, 0700); err != nil {
		return Credentials{}, fmt.Errorf("create adguard data directory: %w", err)
	}
	tempPath := filepath.Join(m.dataDir, ".credentials.json.tmp")
	if err := writeCredentials(tempPath, credentials); err != nil {
		return Credentials{}, err
	}
	defer os.Remove(tempPath)

	installRequest := map[string]any{
		"web":      map[string]any{"ip": "0.0.0.0", "port": 80},
		"dns":      map[string]any{"ip": "0.0.0.0", "port": 53},
		"username": credentials.Username,
		"password": credentials.Password,
	}
	if err := m.request(ctx, http.MethodPost, m.installerURL+"/control/install/configure", installRequest, nil, nil); err != nil {
		return Credentials{}, fmt.Errorf("configure adguard: %w", err)
	}
	if err := os.Rename(tempPath, filepath.Join(m.dataDir, "credentials.json")); err != nil {
		return Credentials{}, fmt.Errorf("activate adguard credentials: %w", err)
	}
	return credentials, nil
}

func (m *Manager) waitUntilInstallerReady(ctx context.Context) error {
	var lastErr error
	for attempt := 0; attempt < 60; attempt++ {
		lastErr = m.request(ctx, http.MethodGet, m.installerURL+"/control/install/get_addresses", nil, nil, nil)
		if lastErr == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for AdGuard installer: %w", ctx.Err())
		case <-time.After(m.retryDelay):
		}
	}
	return fmt.Errorf("adguard installer did not become ready: %w", lastErr)
}

func (m *Manager) waitUntilReady(ctx context.Context, credentials Credentials) error {
	var lastErr error
	for attempt := 0; attempt < 20; attempt++ {
		lastErr = m.request(ctx, http.MethodGet, m.apiURL+"/control/status", nil, nil, &credentials)
		if lastErr == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
	return fmt.Errorf("adguard did not become ready: %w", lastErr)
}

func (m *Manager) configureUpstream(ctx context.Context, credentials Credentials, blockPageIP string) error {
	testRequest := map[string]any{
		"bootstrap_dns":    []string{},
		"upstream_dns":     []string{m.upstream},
		"fallback_dns":     []string{},
		"private_upstream": []string{},
	}
	var testResult map[string]string
	if err := m.request(ctx, http.MethodPost, m.apiURL+"/control/test_upstream_dns", testRequest, &testResult, &credentials); err != nil {
		return fmt.Errorf("test unbound upstream: %w", err)
	}
	if result, ok := testResult[m.upstream]; !ok || result != "OK" {
		return fmt.Errorf("unbound upstream validation failed: %q", result)
	}

	dnsConfig := map[string]any{
		"upstream_dns":         []string{m.upstream},
		"fallback_dns":         []string{},
		"upstream_mode":        "load_balance",
		"protection_enabled":   true,
		"ratelimit":            20,
		"refuse_any":           true,
		"enable_dnssec":        true,
		"edns_cs_enabled":      false,
		"cache_enabled":        true,
		"cache_size":           4 * 1024 * 1024,
		"cache_ttl_min":        0,
		"cache_ttl_max":        0,
		"cache_optimistic":     false,
		"blocked_response_ttl": 10,
		"upstream_timeout":     10,
	}
	if blockPageIP != "" {
		dnsConfig["blocking_mode"] = "custom_ip"
		dnsConfig["blocking_ipv4"] = blockPageIP
		// AdGuard's dns_config validator requires blocking_ipv6 to be a
		// valid IPv6 address whenever blocking_mode is custom_ip, even
		// though RootGuard's DNS bind address (and this blockpage) is
		// IPv4-only. Point it at loopback: an IPv6 client hitting a
		// blocked domain fails closed instead of reaching the real site.
		dnsConfig["blocking_ipv6"] = "::1"
	}
	if err := m.request(ctx, http.MethodPost, m.apiURL+"/control/dns_config", dnsConfig, nil, &credentials); err != nil {
		return fmt.Errorf("configure unbound upstream: %w", err)
	}
	filteringConfig := map[string]any{"enabled": true, "interval": 24}
	if err := m.request(ctx, http.MethodPost, m.apiURL+"/control/filtering/config", filteringConfig, nil, &credentials); err != nil {
		return fmt.Errorf("enable adguard filtering: %w", err)
	}
	return nil
}

func (m *Manager) FilterReport(ctx context.Context) (FilterReport, error) {
	credentials, err := m.loadCredentials()
	if err != nil {
		return FilterReport{}, err
	}

	report := FilterReport{
		Checks:    make([]FilterCheck, 0, len(filterProbes)),
		CheckedAt: time.Now().UTC(),
	}
	for _, probe := range filterProbes {
		var result struct {
			Reason string `json:"reason"`
			Rules  []struct {
				Text string `json:"text"`
			} `json:"rules"`
		}
		endpoint := m.apiURL + "/control/filtering/check_host?name=" + url.QueryEscape(probe.host)
		if err := m.request(ctx, http.MethodGet, endpoint, nil, &result, &credentials); err != nil {
			return FilterReport{}, fmt.Errorf("check adguard filter for %s: %w", probe.host, err)
		}
		blocked := filterReasonBlocks(result.Reason)
		check := FilterCheck{
			Host:            probe.host,
			Category:        probe.category,
			ExpectedBlocked: probe.expectedBlocked,
			Blocked:         blocked,
			Reason:          result.Reason,
		}
		if len(result.Rules) > 0 {
			check.MatchedRule = result.Rules[0].Text
		}
		report.Checks = append(report.Checks, check)
		if blocked {
			report.Blocked++
		}
		if probe.expectedBlocked {
			report.Expected++
			if blocked {
				report.Passed++
			}
		}
	}
	return report, nil
}

func filterReasonBlocks(reason string) bool {
	switch reason {
	case "FilteredBlackList", "FilteredBlockedService", "SafeBrowsing", "Parental":
		return true
	default:
		return false
	}
}

func (m *Manager) request(ctx context.Context, method, url string, body, result any, credentials *Credentials) error {
	var requestBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		requestBody = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, requestBody)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if credentials != nil {
		req.SetBasicAuth(credentials.Username, credentials.Password)
	}
	response, err := m.http.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("adguard returned %d: %s", response.StatusCode, strings.TrimSpace(string(message)))
	}
	if result != nil {
		if err := json.NewDecoder(response.Body).Decode(result); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) loadCredentials() (Credentials, error) {
	data, err := os.ReadFile(filepath.Join(m.dataDir, "credentials.json"))
	if err != nil {
		return Credentials{}, err
	}
	var credentials Credentials
	if err := json.Unmarshal(data, &credentials); err != nil {
		return Credentials{}, fmt.Errorf("decode adguard credentials: %w", err)
	}
	if credentials.Username == "" || credentials.Password == "" {
		return Credentials{}, errors.New("adguard credentials are incomplete")
	}
	return credentials, nil
}

func generateCredentials() (Credentials, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return Credentials{}, err
	}
	return Credentials{
		Username: "rootguard",
		Password: base64.RawURLEncoding.EncodeToString(secret),
	}, nil
}

func writeCredentials(path string, credentials Credentials) error {
	data, err := json.MarshalIndent(credentials, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write adguard credentials: %w", err)
	}
	return nil
}
