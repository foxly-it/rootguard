package coreclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("rootguard core returned %d: %s", e.StatusCode, e.Message)
}

func New(baseURL, token string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		http:    &http.Client{Timeout: 20 * time.Second},
	}
}

type Dashboard struct {
	Docker struct {
		CPU              float64 `json:"cpu"`
		Memory           uint64  `json:"memory"`
		MetricsAvailable bool    `json:"metrics_available"`
		Containers       int     `json:"containers"`
		Status           string  `json:"status"`
	} `json:"docker"`
	DNS struct {
		Status   string `json:"status"`
		Resolver string `json:"resolver"`
		DNSSEC   bool   `json:"dnssec"`
	} `json:"dns"`
}

type Service struct {
	Name         string   `json:"name"`
	DisplayName  string   `json:"displayName"`
	Description  string   `json:"description"`
	Status       string   `json:"status"`
	Health       string   `json:"health"`
	Image        string   `json:"image,omitempty"`
	ImageID      string   `json:"imageId,omitempty"`
	StartedAt    string   `json:"startedAt,omitempty"`
	RestartCount int      `json:"restartCount"`
	Ports        []string `json:"ports,omitempty"`
}

type ServiceLogs struct {
	Service     string   `json:"service"`
	Lines       []string `json:"lines"`
	Tail        int      `json:"tail"`
	Since       string   `json:"since"`
	Truncated   bool     `json:"truncated"`
	Redacted    bool     `json:"redacted"`
	Description string   `json:"description"`
}

type ServiceActionResponse struct {
	Service string `json:"service"`
	Action  string `json:"action"`
	Status  string `json:"status"`
}

type UnboundSettings struct {
	QnameMinimisation         bool                       `json:"qname_minimisation"`
	Prefetch                  bool                       `json:"prefetch"`
	PrefetchKey               bool                       `json:"prefetch_key"`
	AggressiveNSEC            bool                       `json:"aggressive_nsec"`
	EDNSBufferSize            int                        `json:"edns_buffer_size"`
	LogVerbosity              int                        `json:"log_verbosity"`
	ServeExpired              bool                       `json:"serve_expired"`
	ServeExpiredTTL           int                        `json:"serve_expired_ttl"`
	ServeExpiredClientTimeout int                        `json:"serve_expired_client_timeout"`
	CacheMinTTL               int                        `json:"cache_min_ttl"`
	CacheMaxTTL               int                        `json:"cache_max_ttl"`
	Threads                   int                        `json:"threads"`
	ResourceProfile           string                     `json:"resource_profile"`
	NetworkMode               string                     `json:"network_mode"`
	ForwardZones              []UnboundForwardZone       `json:"forward_zones"`
	PrivateDomains            []string                   `json:"private_domains"`
	ReverseZones              []UnboundReverseZonePolicy `json:"reverse_zones"`
}

type UnboundForwardZone struct {
	Name                  string   `json:"name"`
	Servers               []string `json:"servers"`
	ForwardFirst          bool     `json:"forward_first"`
	AllowUnsigned         bool     `json:"allow_unsigned"`
	AllowPrivateAddresses bool     `json:"allow_private_addresses"`
}

type UnboundDiagnosticLoggingStatus struct {
	Active    bool       `json:"active"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	Level     int        `json:"level"`
}

type UnboundReverseZonePolicy struct {
	Network string `json:"network"`
	Mode    string `json:"mode"`
}

type UnboundForwardTargetCheck struct {
	Zone      string `json:"zone"`
	Address   string `json:"address"`
	Reachable bool   `json:"reachable"`
	Detail    string `json:"detail"`
}

type UnboundNetworkCapabilities struct {
	IPv4Available bool      `json:"ipv4_available"`
	IPv4Detail    string    `json:"ipv4_detail"`
	IPv6Available bool      `json:"ipv6_available"`
	IPv6Detail    string    `json:"ipv6_detail"`
	CheckedAt     time.Time `json:"checked_at"`
}

type UnboundActiveConfiguration struct {
	BaseConfig    string    `json:"base_config"`
	ManagedConfig string    `json:"managed_config"`
	CustomConfig  string    `json:"custom_config"`
	CheckedAt     time.Time `json:"checked_at"`
}

type UnboundChange struct {
	Field  string `json:"field"`
	Before string `json:"before"`
	After  string `json:"after"`
}

type UnboundPreview struct {
	Changed        bool            `json:"changed"`
	Changes        []UnboundChange `json:"changes"`
	RenderedConfig string          `json:"rendered_config"`
}

type UnboundHistoryEntry struct {
	ID           string          `json:"id"`
	CreatedAt    time.Time       `json:"created_at"`
	Settings     UnboundSettings `json:"settings"`
	Config       string          `json:"config,omitempty"`
	CustomConfig string          `json:"custom_config,omitempty"`
}

type UnboundDiagnosticCheck struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail"`
}

type UnboundDiagnosticReport struct {
	Healthy   bool                     `json:"healthy"`
	CheckedAt time.Time                `json:"checked_at"`
	Checks    []UnboundDiagnosticCheck `json:"checks"`
}

type UnboundPreset struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	BestFor     string          `json:"best_for"`
	Settings    UnboundSettings `json:"settings"`
}

type UnboundRecommendation struct {
	ID          string `json:"id"`
	Severity    string `json:"severity"`
	Field       string `json:"field,omitempty"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Suggestion  string `json:"suggestion"`
}

type UnboundAdvice struct {
	Status          string                  `json:"status"`
	Recommendations []UnboundRecommendation `json:"recommendations"`
}

type UnboundCustomDocument struct {
	Content  string `json:"content"`
	MaxBytes int    `json:"max_bytes"`
}

type UnboundCustomAdvice struct {
	ID          string `json:"id"`
	Severity    string `json:"severity"`
	Line        int    `json:"line,omitempty"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Suggestion  string `json:"suggestion"`
}

type UnboundCustomPreview struct {
	Changed    bool                  `json:"changed"`
	Content    string                `json:"content"`
	Validation string                `json:"validation"`
	Advice     []UnboundCustomAdvice `json:"advice"`
}

type UnboundDirectiveReference struct {
	Name        string `json:"name"`
	Section     string `json:"section"`
	Example     string `json:"example"`
	Description string `json:"description"`
	Risk        string `json:"risk"`
}

type AdGuardStatus struct {
	Configured         bool    `json:"configured"`
	Healthy            bool    `json:"healthy"`
	Upstream           string  `json:"upstream"`
	UpstreamReady      bool    `json:"upstream_ready"`
	StatsAvailable     bool    `json:"stats_available"`
	Queries            uint64  `json:"queries"`
	Blocked            uint64  `json:"blocked"`
	AverageResponse    float64 `json:"average_response_seconds"`
	BestPracticesReady bool    `json:"best_practices_ready"`
}

type AdGuardFilterCheck struct {
	Host            string `json:"host"`
	Category        string `json:"category"`
	ExpectedBlocked bool   `json:"expected_blocked"`
	Blocked         bool   `json:"blocked"`
	Reason          string `json:"reason"`
	MatchedRule     string `json:"matched_rule,omitempty"`
}

type AdGuardFilterReport struct {
	Checks    []AdGuardFilterCheck `json:"checks"`
	Blocked   int                  `json:"blocked"`
	Expected  int                  `json:"expected"`
	Passed    int                  `json:"passed"`
	CheckedAt time.Time            `json:"checked_at"`
}

type InstallationConfig struct {
	DNSBindAddress string `json:"dns_bind_address"`
	DNSPort        int    `json:"dns_port"`
	AdGuardChannel string `json:"adguard_channel"`
}

type InstallationCheck struct {
	ID      string `json:"id"`
	Code    string `json:"code"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
	Action  string `json:"action,omitempty"`
}

type InstallationPreflight struct {
	Ready  bool                `json:"ready"`
	Config InstallationConfig  `json:"config"`
	Checks []InstallationCheck `json:"checks"`
}

type InstallationStep struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type InstallationDiagnostic struct {
	Code      string `json:"code"`
	Phase     string `json:"phase"`
	Message   string `json:"message"`
	Detail    string `json:"detail,omitempty"`
	Action    string `json:"action"`
	Retryable bool   `json:"retryable"`
}

type InstallationStatus struct {
	State      string                  `json:"state"`
	Config     *InstallationConfig     `json:"config,omitempty"`
	Steps      []InstallationStep      `json:"steps"`
	Error      string                  `json:"error,omitempty"`
	Diagnostic *InstallationDiagnostic `json:"diagnostic,omitempty"`
	UpdatedAt  time.Time               `json:"updated_at"`
}

type UpdateServiceStatus struct {
	Name            string    `json:"name"`
	DisplayName     string    `json:"display_name"`
	CurrentImage    string    `json:"current_image,omitempty"`
	TargetImage     string    `json:"target_image"`
	CurrentID       string    `json:"current_id,omitempty"`
	CandidateID     string    `json:"candidate_id,omitempty"`
	UpdateAvailable bool      `json:"update_available"`
	CheckedAt       time.Time `json:"checked_at,omitempty"`
	Error           string    `json:"error,omitempty"`
}

type UpdateCleanupResult struct {
	RemovedImages  []string `json:"removed_images,omitempty"`
	RemovedVolumes []string `json:"removed_volumes,omitempty"`
	Skipped        []string `json:"skipped,omitempty"`
}

type UpdateHistoryEntry struct {
	Service   string              `json:"service,omitempty"`
	Outcome   string              `json:"outcome"`
	FromID    string              `json:"from_id,omitempty"`
	ToID      string              `json:"to_id,omitempty"`
	FromIDs   map[string]string   `json:"from_ids,omitempty"`
	ToIDs     map[string]string   `json:"to_ids,omitempty"`
	Message   string              `json:"message"`
	Cleanup   UpdateCleanupResult `json:"cleanup"`
	CreatedAt time.Time           `json:"created_at"`
}

type UpdateStatus struct {
	State         string                `json:"state"`
	ActiveService string                `json:"active_service,omitempty"`
	Message       string                `json:"message"`
	Services      []UpdateServiceStatus `json:"services"`
	History       []UpdateHistoryEntry  `json:"history,omitempty"`
	UpdatedAt     time.Time             `json:"updated_at"`
}

type ControlPlaneUpdateStatus struct {
	State     string                `json:"state"`
	Message   string                `json:"message"`
	Services  []UpdateServiceStatus `json:"services"`
	History   []UpdateHistoryEntry  `json:"history,omitempty"`
	UpdatedAt time.Time             `json:"updated_at"`
}

func (c *Client) Dashboard(ctx context.Context) (Dashboard, error) {
	var result Dashboard
	err := c.do(ctx, http.MethodGet, "/api/dashboard", nil, &result)
	return result, err
}

func (c *Client) System(ctx context.Context) (map[string]string, error) {
	var result map[string]string
	err := c.do(ctx, http.MethodGet, "/api/system", nil, &result)
	return result, err
}

func (c *Client) Services(ctx context.Context) ([]Service, error) {
	var result []Service
	err := c.do(ctx, http.MethodGet, "/api/services", nil, &result)
	return result, err
}

func (c *Client) ServiceLogs(ctx context.Context, service string) (ServiceLogs, error) {
	var result ServiceLogs
	err := c.do(ctx, http.MethodGet, "/api/services/"+service+"/logs", nil, &result)
	return result, err
}

func (c *Client) ServiceAction(ctx context.Context, service, action string) (ServiceActionResponse, error) {
	var result ServiceActionResponse
	err := c.do(ctx, http.MethodPost, "/api/services/"+service+"/"+action, nil, &result)
	return result, err
}

func (c *Client) UnboundSettings(ctx context.Context) (UnboundSettings, error) {
	var result UnboundSettings
	err := c.do(ctx, http.MethodGet, "/api/unbound/settings", nil, &result)
	return result, err
}

func (c *Client) UnboundActiveConfiguration(ctx context.Context) (UnboundActiveConfiguration, error) {
	var result UnboundActiveConfiguration
	err := c.do(ctx, http.MethodGet, "/api/unbound/config", nil, &result)
	return result, err
}

func (c *Client) UpdateUnboundSettings(ctx context.Context, settings UnboundSettings) (UnboundSettings, error) {
	var result UnboundSettings
	err := c.do(ctx, http.MethodPut, "/api/unbound/settings", settings, &result)
	return result, err
}

func (c *Client) PreviewUnboundSettings(ctx context.Context, settings UnboundSettings) (UnboundPreview, error) {
	var result UnboundPreview
	err := c.do(ctx, http.MethodPost, "/api/unbound/preview", settings, &result)
	return result, err
}

func (c *Client) UnboundHistory(ctx context.Context) ([]UnboundHistoryEntry, error) {
	var result []UnboundHistoryEntry
	err := c.do(ctx, http.MethodGet, "/api/unbound/history", nil, &result)
	return result, err
}

func (c *Client) RestoreUnboundVersion(ctx context.Context, id string) (UnboundSettings, error) {
	var result UnboundSettings
	err := c.do(ctx, http.MethodPost, "/api/unbound/history/"+id+"/restore", nil, &result)
	return result, err
}

func (c *Client) UnboundDiagnostics(ctx context.Context) (UnboundDiagnosticReport, error) {
	var result UnboundDiagnosticReport
	err := c.do(ctx, http.MethodGet, "/api/unbound/diagnostics", nil, &result)
	return result, err
}

func (c *Client) UnboundDiagnosticLoggingStatus(ctx context.Context) (UnboundDiagnosticLoggingStatus, error) {
	var result UnboundDiagnosticLoggingStatus
	err := c.do(ctx, http.MethodGet, "/api/unbound/diagnostic-logging", nil, &result)
	return result, err
}

func (c *Client) StartUnboundDiagnosticLogging(ctx context.Context) (UnboundDiagnosticLoggingStatus, error) {
	var result UnboundDiagnosticLoggingStatus
	err := c.do(ctx, http.MethodPost, "/api/unbound/diagnostic-logging", nil, &result)
	return result, err
}

func (c *Client) StopUnboundDiagnosticLogging(ctx context.Context) (UnboundDiagnosticLoggingStatus, error) {
	var result UnboundDiagnosticLoggingStatus
	err := c.do(ctx, http.MethodDelete, "/api/unbound/diagnostic-logging", nil, &result)
	return result, err
}

func (c *Client) UnboundPresets(ctx context.Context) ([]UnboundPreset, error) {
	var result []UnboundPreset
	err := c.do(ctx, http.MethodGet, "/api/unbound/presets", nil, &result)
	return result, err
}

func (c *Client) UnboundAdvice(ctx context.Context, settings UnboundSettings) (UnboundAdvice, error) {
	var result UnboundAdvice
	err := c.do(ctx, http.MethodPost, "/api/unbound/advice", settings, &result)
	return result, err
}

func (c *Client) CheckUnboundForwardTargets(ctx context.Context, zones []UnboundForwardZone) ([]UnboundForwardTargetCheck, error) {
	var result []UnboundForwardTargetCheck
	err := c.do(ctx, http.MethodPost, "/api/unbound/forward-check", map[string]any{"zones": zones}, &result)
	return result, err
}

func (c *Client) UnboundNetworkCapabilities(ctx context.Context) (UnboundNetworkCapabilities, error) {
	var result UnboundNetworkCapabilities
	err := c.do(ctx, http.MethodGet, "/api/unbound/network-capabilities", nil, &result)
	return result, err
}

func (c *Client) UnboundCustom(ctx context.Context) (UnboundCustomDocument, error) {
	var result UnboundCustomDocument
	err := c.do(ctx, http.MethodGet, "/api/unbound/custom", nil, &result)
	return result, err
}

func (c *Client) PreviewUnboundCustom(ctx context.Context, content string) (UnboundCustomPreview, error) {
	var result UnboundCustomPreview
	err := c.do(ctx, http.MethodPost, "/api/unbound/custom/preview", map[string]string{"content": content}, &result)
	return result, err
}

func (c *Client) UpdateUnboundCustom(ctx context.Context, content string) (UnboundCustomDocument, error) {
	var result UnboundCustomDocument
	err := c.do(ctx, http.MethodPut, "/api/unbound/custom", map[string]string{"content": content}, &result)
	return result, err
}

func (c *Client) UnboundDirectives(ctx context.Context) ([]UnboundDirectiveReference, error) {
	var result []UnboundDirectiveReference
	err := c.do(ctx, http.MethodGet, "/api/unbound/directives", nil, &result)
	return result, err
}

func (c *Client) AdGuardStatus(ctx context.Context) (AdGuardStatus, error) {
	var result AdGuardStatus
	err := c.do(ctx, http.MethodGet, "/api/adguard/status", nil, &result)
	return result, err
}

func (c *Client) BootstrapAdGuard(ctx context.Context) (AdGuardStatus, error) {
	var result AdGuardStatus
	err := c.do(ctx, http.MethodPost, "/api/adguard/bootstrap", nil, &result)
	return result, err
}

func (c *Client) AdGuardFilterReport(ctx context.Context) (AdGuardFilterReport, error) {
	var result AdGuardFilterReport
	err := c.do(ctx, http.MethodGet, "/api/adguard/filter-report", nil, &result)
	return result, err
}

func (c *Client) AdGuardUIHandler() http.Handler {
	target, err := url.Parse(c.baseURL)
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "invalid RootGuard Core URL", http.StatusInternalServerError)
		})
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	originalDirector := proxy.Director
	proxy.Director = func(request *http.Request) {
		originalDirector(request)
		path := strings.TrimPrefix(request.URL.Path, "/adguard-ui")
		if path == "" {
			path = "/"
		}
		request.URL.Path = "/api/adguard/ui" + path
		request.URL.RawPath = ""
		request.Host = target.Host
		request.Header.Set("Authorization", "Bearer "+c.token)
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, proxyErr error) {
		http.Error(w, fmt.Sprintf("RootGuard AdGuard UI proxy: %v", proxyErr), http.StatusBadGateway)
	}
	return proxy
}

func (c *Client) InstallationStatus(ctx context.Context) (InstallationStatus, error) {
	var result InstallationStatus
	err := c.do(ctx, http.MethodGet, "/api/installation", nil, &result)
	return result, err
}

func (c *Client) InstallationPreflight(ctx context.Context, config InstallationConfig) (InstallationPreflight, error) {
	var result InstallationPreflight
	err := c.do(ctx, http.MethodPost, "/api/installation/preflight", config, &result)
	return result, err
}

func (c *Client) DeployInstallation(ctx context.Context, config InstallationConfig) (InstallationStatus, error) {
	var result InstallationStatus
	err := c.do(ctx, http.MethodPost, "/api/installation/deploy", config, &result)
	return result, err
}

func (c *Client) UpdateStatus(ctx context.Context) (UpdateStatus, error) {
	var result UpdateStatus
	err := c.do(ctx, http.MethodGet, "/api/updates", nil, &result)
	return result, err
}

func (c *Client) CheckUpdates(ctx context.Context) (UpdateStatus, error) {
	var result UpdateStatus
	err := c.do(ctx, http.MethodPost, "/api/updates/check", nil, &result)
	return result, err
}

func (c *Client) UpdateService(ctx context.Context, service string) (UpdateStatus, error) {
	var result UpdateStatus
	err := c.do(ctx, http.MethodPost, "/api/updates/"+service, nil, &result)
	return result, err
}

func (c *Client) ControlPlaneUpdateStatus(ctx context.Context) (ControlPlaneUpdateStatus, error) {
	var result ControlPlaneUpdateStatus
	err := c.do(ctx, http.MethodGet, "/api/control-plane-updates", nil, &result)
	return result, err
}

func (c *Client) CheckControlPlaneUpdates(ctx context.Context) (ControlPlaneUpdateStatus, error) {
	var result ControlPlaneUpdateStatus
	err := c.do(ctx, http.MethodPost, "/api/control-plane-updates/check", nil, &result)
	return result, err
}

func (c *Client) InstallControlPlaneUpdates(ctx context.Context) (ControlPlaneUpdateStatus, error) {
	var result ControlPlaneUpdateStatus
	err := c.do(ctx, http.MethodPost, "/api/control-plane-updates/install", nil, &result)
	return result, err
}

func (c *Client) do(ctx context.Context, method, path string, body, result any) error {
	var requestBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		requestBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, requestBody)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("rootguard core request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    strings.TrimSpace(string(message)),
		}
	}
	if result == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("decode rootguard core response: %w", err)
	}
	return nil
}
