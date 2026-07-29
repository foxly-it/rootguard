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
	Configured      bool    `json:"configured"`
	Healthy         bool    `json:"healthy"`
	Upstream        string  `json:"upstream"`
	UpstreamReady   bool    `json:"upstream_ready"`
	StatsAvailable  bool    `json:"stats_available"`
	Queries         uint64  `json:"queries"`
	Blocked         uint64  `json:"blocked"`
	AverageResponse float64 `json:"average_response_seconds"`
}

type Manager struct {
	installerURL string
	apiURL       string
	dataDir      string
	upstream     string
	http         *http.Client
	retryDelay   time.Duration
}

func NewManager(installerURL, apiURL, dataDir, upstream string) *Manager {
	return &Manager{
		installerURL: strings.TrimRight(installerURL, "/"),
		apiURL:       strings.TrimRight(apiURL, "/"),
		dataDir:      dataDir,
		upstream:     upstream,
		http:         &http.Client{Timeout: 10 * time.Second},
		retryDelay:   500 * time.Millisecond,
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

	if err := m.request(ctx, http.MethodGet, m.apiURL+"/control/status", nil, nil, &credentials); err != nil {
		return Status{}, fmt.Errorf("adguard status: %w", err)
	}

	var dnsInfo struct {
		UpstreamDNS []string `json:"upstream_dns"`
		FallbackDNS []string `json:"fallback_dns"`
	}
	if err := m.request(ctx, http.MethodGet, m.apiURL+"/control/dns_info", nil, &dnsInfo, &credentials); err != nil {
		return Status{}, fmt.Errorf("adguard dns info: %w", err)
	}

	status := Status{
		Configured: true,
		Healthy:    true,
		Upstream:   m.upstream,
		UpstreamReady: len(dnsInfo.UpstreamDNS) == 1 &&
			dnsInfo.UpstreamDNS[0] == m.upstream && len(dnsInfo.FallbackDNS) == 0,
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
	return status, nil
}

func (m *Manager) Bootstrap(ctx context.Context) (Status, error) {
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
	if err := m.configureUpstream(ctx, credentials); err != nil {
		return Status{}, err
	}
	return m.Status(ctx)
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

func (m *Manager) configureUpstream(ctx context.Context, credentials Credentials) error {
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
		"upstream_dns":  []string{m.upstream},
		"fallback_dns":  []string{},
		"upstream_mode": "load_balance",
	}
	if err := m.request(ctx, http.MethodPost, m.apiURL+"/control/dns_config", dnsConfig, nil, &credentials); err != nil {
		return fmt.Errorf("configure unbound upstream: %w", err)
	}
	return nil
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
