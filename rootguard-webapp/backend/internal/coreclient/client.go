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
		CollectedAt      int64   `json:"collected_at"`
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
	Version      string   `json:"version,omitempty"`
	Revision     string   `json:"revision,omitempty"`
	Created      string   `json:"created,omitempty"`
	Source       string   `json:"source,omitempty"`
	Immutable    bool     `json:"immutable"`
	Metadata     string   `json:"metadata"`
	Attestation  string   `json:"attestation"`
	AttestedAt   string   `json:"attestedAt,omitempty"`
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
	LocalZones                []UnboundLocalZone         `json:"local_zones"`
}

type UnboundForwardZone struct {
	Name                  string   `json:"name"`
	Servers               []string `json:"servers"`
	ForwardFirst          bool     `json:"forward_first"`
	AllowUnsigned         bool     `json:"allow_unsigned"`
	AllowPrivateAddresses bool     `json:"allow_private_addresses"`
}

type UnboundLocalHost struct {
	Hostname string `json:"hostname"`
	IPv4     string `json:"ipv4,omitempty"`
	IPv6     string `json:"ipv6,omitempty"`
	PTR      bool   `json:"ptr"`
}

type UnboundLocalZone struct {
	Name  string             `json:"name"`
	Hosts []UnboundLocalHost `json:"hosts"`
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

type UnboundConfigBundle struct {
	SchemaVersion int             `json:"schema_version"`
	ExportedAt    time.Time       `json:"exported_at"`
	Settings      UnboundSettings `json:"settings"`
	CustomConfig  string          `json:"custom_config"`
}

type UnboundBundlePreview struct {
	Changed        bool            `json:"changed"`
	Changes        []UnboundChange `json:"changes"`
	CustomChanged  bool            `json:"custom_changed"`
	RenderedConfig string          `json:"rendered_config"`
}

type UnboundImportFinding struct {
	Section     string `json:"section"`
	Line        int    `json:"line"`
	Directive   string `json:"directive"`
	Value       string `json:"value,omitempty"`
	Disposition string `json:"disposition"`
	Detail      string `json:"detail"`
}

type UnboundImportResult struct {
	Findings      []UnboundImportFinding `json:"findings"`
	Settings      UnboundSettings        `json:"settings"`
	CustomAdopted string                 `json:"custom_adopted"`
}

type UnboundDirectiveReference struct {
	Name        string `json:"name"`
	Section     string `json:"section"`
	Example     string `json:"example"`
	Description string `json:"description"`
	Risk        string `json:"risk"`
}

type AdGuardStatus struct {
	Configured                   bool    `json:"configured"`
	Healthy                      bool    `json:"healthy"`
	Version                      string  `json:"version,omitempty"`
	Upstream                     string  `json:"upstream"`
	UpstreamReady                bool    `json:"upstream_ready"`
	StatsAvailable               bool    `json:"stats_available"`
	Queries                      uint64  `json:"queries"`
	Blocked                      uint64  `json:"blocked"`
	AverageResponse              float64 `json:"average_response_seconds"`
	BestPracticesReady           bool    `json:"best_practices_ready"`
	FilteringEnabled             bool    `json:"filtering_enabled"`
	ActiveFilterLists            int     `json:"active_filter_lists"`
	TotalFilterLists             int     `json:"total_filter_lists"`
	ProtectionEnabled            bool    `json:"protection_enabled"`
	ProtectionDisabledDurationMs int64   `json:"protection_disabled_duration_ms"`
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
	DNSBindAddress   string `json:"dns_bind_address"`
	DNSPort          int    `json:"dns_port"`
	AdGuardChannel   string `json:"adguard_channel"`
	BlockpageEnabled bool   `json:"blockpage_enabled"`
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

type BackupSettings struct {
	RetentionPerService int `json:"retention_per_service"`
}

type BackupServiceUsage struct {
	Service  string     `json:"service"`
	Count    int        `json:"count"`
	Bytes    int64      `json:"bytes"`
	OldestAt *time.Time `json:"oldest_at,omitempty"`
	NewestAt *time.Time `json:"newest_at,omitempty"`
}

type BackupStatus struct {
	Settings       BackupSettings       `json:"settings"`
	Count          int                  `json:"count"`
	ManagedBytes   int64                `json:"managed_bytes"`
	UnmanagedBytes int64                `json:"unmanaged_bytes"`
	Services       []BackupServiceUsage `json:"services"`
	LastError      string               `json:"last_error,omitempty"`
}

type BackupRestorePreview struct {
	SchemaVersion int                   `json:"schema_version"`
	CreatedAt     time.Time             `json:"created_at"`
	FileCount     int                   `json:"file_count"`
	ExpandedBytes int64                 `json:"expanded_bytes"`
	Config        InstallationConfig    `json:"config"`
	Preflight     InstallationPreflight `json:"preflight"`
}

type CleanupResource struct {
	Kind           string `json:"kind"`
	ID             string `json:"id"`
	EstimatedBytes int64  `json:"estimated_bytes"`
}

type CleanupPreview struct {
	Resources      []CleanupResource `json:"resources"`
	Skipped        []string          `json:"skipped,omitempty"`
	EstimatedBytes int64             `json:"estimated_bytes"`
}

type ControlPlaneUpdateStatus struct {
	State     string                `json:"state"`
	Message   string                `json:"message"`
	Services  []UpdateServiceStatus `json:"services"`
	History   []UpdateHistoryEntry  `json:"history,omitempty"`
	UpdatedAt time.Time             `json:"updated_at"`
}

func (c *Client) Dashboard(ctx context.Context) (Dashboard, error) {
	return doJSON[Dashboard](ctx, c, http.MethodGet, "/api/dashboard", nil)
}

func (c *Client) System(ctx context.Context) (map[string]string, error) {
	return doJSON[map[string]string](ctx, c, http.MethodGet, "/api/system", nil)
}

func (c *Client) Services(ctx context.Context) ([]Service, error) {
	return doJSON[[]Service](ctx, c, http.MethodGet, "/api/services", nil)
}

func (c *Client) ServiceLogs(ctx context.Context, service string) (ServiceLogs, error) {
	return doJSON[ServiceLogs](ctx, c, http.MethodGet, "/api/services/"+service+"/logs", nil)
}

func (c *Client) ServiceAction(ctx context.Context, service, action string) (ServiceActionResponse, error) {
	return doJSON[ServiceActionResponse](ctx, c, http.MethodPost, "/api/services/"+service+"/"+action, nil)
}

func (c *Client) UnboundSettings(ctx context.Context) (UnboundSettings, error) {
	return doJSON[UnboundSettings](ctx, c, http.MethodGet, "/api/unbound/settings", nil)
}

func (c *Client) UnboundActiveConfiguration(ctx context.Context) (UnboundActiveConfiguration, error) {
	return doJSON[UnboundActiveConfiguration](ctx, c, http.MethodGet, "/api/unbound/config", nil)
}

func (c *Client) UpdateUnboundSettings(ctx context.Context, settings UnboundSettings) (UnboundSettings, error) {
	return doJSON[UnboundSettings](ctx, c, http.MethodPut, "/api/unbound/settings", settings)
}

func (c *Client) PreviewUnboundSettings(ctx context.Context, settings UnboundSettings) (UnboundPreview, error) {
	return doJSON[UnboundPreview](ctx, c, http.MethodPost, "/api/unbound/preview", settings)
}

func (c *Client) UnboundHistory(ctx context.Context) ([]UnboundHistoryEntry, error) {
	return doJSON[[]UnboundHistoryEntry](ctx, c, http.MethodGet, "/api/unbound/history", nil)
}

func (c *Client) RestoreUnboundVersion(ctx context.Context, id string) (UnboundSettings, error) {
	return doJSON[UnboundSettings](ctx, c, http.MethodPost, "/api/unbound/history/"+id+"/restore", nil)
}

func (c *Client) UnboundDiagnostics(ctx context.Context) (UnboundDiagnosticReport, error) {
	return doJSON[UnboundDiagnosticReport](ctx, c, http.MethodGet, "/api/unbound/diagnostics", nil)
}

// UnboundPathDiagnostics reuses UnboundDiagnosticReport/UnboundDiagnosticCheck
// - same shape, different checks (resolution and DNSSEC rejection through
// AdGuard's own listener rather than Unbound's).
func (c *Client) UnboundPathDiagnostics(ctx context.Context) (UnboundDiagnosticReport, error) {
	return doJSON[UnboundDiagnosticReport](ctx, c, http.MethodGet, "/api/unbound/path-diagnostics", nil)
}

func (c *Client) UnboundDiagnosticLoggingStatus(ctx context.Context) (UnboundDiagnosticLoggingStatus, error) {
	return doJSON[UnboundDiagnosticLoggingStatus](ctx, c, http.MethodGet, "/api/unbound/diagnostic-logging", nil)
}

func (c *Client) StartUnboundDiagnosticLogging(ctx context.Context) (UnboundDiagnosticLoggingStatus, error) {
	return doJSON[UnboundDiagnosticLoggingStatus](ctx, c, http.MethodPost, "/api/unbound/diagnostic-logging", nil)
}

func (c *Client) StopUnboundDiagnosticLogging(ctx context.Context) (UnboundDiagnosticLoggingStatus, error) {
	return doJSON[UnboundDiagnosticLoggingStatus](ctx, c, http.MethodDelete, "/api/unbound/diagnostic-logging", nil)
}

func (c *Client) UnboundPresets(ctx context.Context) ([]UnboundPreset, error) {
	return doJSON[[]UnboundPreset](ctx, c, http.MethodGet, "/api/unbound/presets", nil)
}

func (c *Client) UnboundAdvice(ctx context.Context, settings UnboundSettings) (UnboundAdvice, error) {
	return doJSON[UnboundAdvice](ctx, c, http.MethodPost, "/api/unbound/advice", settings)
}

func (c *Client) CheckUnboundForwardTargets(ctx context.Context, zones []UnboundForwardZone) ([]UnboundForwardTargetCheck, error) {
	return doJSON[[]UnboundForwardTargetCheck](ctx, c, http.MethodPost, "/api/unbound/forward-check", map[string]any{"zones": zones})
}

type DiscoveredHost struct {
	Hostname string `json:"hostname"`
	IPv4     string `json:"ipv4"`
	IPv6     string `json:"ipv6,omitempty"`
	MAC      string `json:"mac,omitempty"`
	Active   bool   `json:"active"`
	Source   string `json:"source"`
}

type RouterDiscoveryResult struct {
	Hosts     []DiscoveredHost `json:"hosts"`
	Truncated bool             `json:"truncated"`
	Scanned   int              `json:"scanned,omitempty"`
	Failed    int              `json:"failed,omitempty"`
}

func (c *Client) DiscoverReverseDNSHosts(ctx context.Context, networks []string) (RouterDiscoveryResult, error) {
	return doJSON[RouterDiscoveryResult](ctx, c, http.MethodPost, "/api/router-import/reverse-dns/discover", map[string]any{"networks": networks})
}

func (c *Client) DiscoverFritzBoxHosts(ctx context.Context, address, username, password string) (RouterDiscoveryResult, error) {
	return doJSON[RouterDiscoveryResult](ctx, c, http.MethodPost, "/api/router-import/fritzbox/discover", map[string]string{
		"address": address, "username": username, "password": password,
	})
}

func (c *Client) UnboundNetworkCapabilities(ctx context.Context) (UnboundNetworkCapabilities, error) {
	return doJSON[UnboundNetworkCapabilities](ctx, c, http.MethodGet, "/api/unbound/network-capabilities", nil)
}

func (c *Client) UnboundCustom(ctx context.Context) (UnboundCustomDocument, error) {
	return doJSON[UnboundCustomDocument](ctx, c, http.MethodGet, "/api/unbound/custom", nil)
}

func (c *Client) PreviewUnboundCustom(ctx context.Context, content string) (UnboundCustomPreview, error) {
	return doJSON[UnboundCustomPreview](ctx, c, http.MethodPost, "/api/unbound/custom/preview", map[string]string{"content": content})
}

func (c *Client) UpdateUnboundCustom(ctx context.Context, content string) (UnboundCustomDocument, error) {
	return doJSON[UnboundCustomDocument](ctx, c, http.MethodPut, "/api/unbound/custom", map[string]string{"content": content})
}

func (c *Client) UnboundExport(ctx context.Context) (UnboundConfigBundle, error) {
	return doJSON[UnboundConfigBundle](ctx, c, http.MethodGet, "/api/unbound/export", nil)
}

func (c *Client) PreviewUnboundImport(ctx context.Context, bundle UnboundConfigBundle) (UnboundBundlePreview, error) {
	return doJSON[UnboundBundlePreview](ctx, c, http.MethodPost, "/api/unbound/import/preview", bundle)
}

func (c *Client) ApplyUnboundImport(ctx context.Context, bundle UnboundConfigBundle) (UnboundSettings, error) {
	return doJSON[UnboundSettings](ctx, c, http.MethodPost, "/api/unbound/import", bundle)
}

func (c *Client) ClassifyUnboundImportConf(ctx context.Context, content string) (UnboundImportResult, error) {
	return doJSON[UnboundImportResult](ctx, c, http.MethodPost, "/api/unbound/import-conf", map[string]string{"content": content})
}

func (c *Client) UnboundDirectives(ctx context.Context) ([]UnboundDirectiveReference, error) {
	return doJSON[[]UnboundDirectiveReference](ctx, c, http.MethodGet, "/api/unbound/directives", nil)
}

func (c *Client) AdGuardStatus(ctx context.Context) (AdGuardStatus, error) {
	return doJSON[AdGuardStatus](ctx, c, http.MethodGet, "/api/adguard/status", nil)
}

func (c *Client) SetAdGuardFiltering(ctx context.Context, enabled bool) (AdGuardStatus, error) {
	body := struct {
		Enabled bool `json:"enabled"`
	}{Enabled: enabled}
	return doJSON[AdGuardStatus](ctx, c, http.MethodPost, "/api/adguard/filtering", body)
}

func (c *Client) BootstrapAdGuard(ctx context.Context) (AdGuardStatus, error) {
	return doJSON[AdGuardStatus](ctx, c, http.MethodPost, "/api/adguard/bootstrap", nil)
}

func (c *Client) SetAdGuardProtection(ctx context.Context, enabled bool, durationSeconds int64) (AdGuardStatus, error) {
	body := struct {
		Enabled         bool  `json:"enabled"`
		DurationSeconds int64 `json:"duration_seconds"`
	}{Enabled: enabled, DurationSeconds: durationSeconds}
	return doJSON[AdGuardStatus](ctx, c, http.MethodPost, "/api/adguard/protection", body)
}

func (c *Client) AdGuardFilterReport(ctx context.Context) (AdGuardFilterReport, error) {
	return doJSON[AdGuardFilterReport](ctx, c, http.MethodGet, "/api/adguard/filter-report", nil)
}

func (c *Client) AdGuardUIHandler() http.Handler {
	target, err := url.Parse(c.baseURL)
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "invalid RootGuard Core URL", http.StatusInternalServerError)
		})
	}
	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			// Director never called this, so client-supplied X-Forwarded-*
			// headers were passed through unsanitized; Rewrite's
			// replacement strips them before setting fresh values, closing
			// that spoofing path.
			pr.SetXForwarded()
			path := strings.TrimPrefix(pr.In.URL.Path, "/adguard-ui")
			if path == "" {
				path = "/"
			}
			pr.Out.URL.Path = "/api/adguard/ui" + path
			pr.Out.URL.RawPath = ""
			pr.Out.Host = target.Host
			pr.Out.Header.Set("Authorization", "Bearer "+c.token)
		},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, proxyErr error) {
			http.Error(w, fmt.Sprintf("RootGuard AdGuard UI proxy: %v", proxyErr), http.StatusBadGateway)
		},
	}
	return proxy
}

func (c *Client) InstallationStatus(ctx context.Context) (InstallationStatus, error) {
	return doJSON[InstallationStatus](ctx, c, http.MethodGet, "/api/installation", nil)
}

func (c *Client) InstallationPreflight(ctx context.Context, config InstallationConfig) (InstallationPreflight, error) {
	return doJSON[InstallationPreflight](ctx, c, http.MethodPost, "/api/installation/preflight", config)
}

func (c *Client) DeployInstallation(ctx context.Context, config InstallationConfig) (InstallationStatus, error) {
	return doJSON[InstallationStatus](ctx, c, http.MethodPost, "/api/installation/deploy", config)
}

func (c *Client) UpdateStatus(ctx context.Context) (UpdateStatus, error) {
	return doJSON[UpdateStatus](ctx, c, http.MethodGet, "/api/updates", nil)
}

func (c *Client) BackupStatus(ctx context.Context) (BackupStatus, error) {
	return doJSON[BackupStatus](ctx, c, http.MethodGet, "/api/backups", nil)
}

func (c *Client) SetBackupRetention(ctx context.Context, retention int) (BackupStatus, error) {
	return doJSON[BackupStatus](ctx, c, http.MethodPut, "/api/backups/settings", map[string]int{"retention_per_service": retention})
}

func (c *Client) CleanupPreview(ctx context.Context) (CleanupPreview, error) {
	return doJSON[CleanupPreview](ctx, c, http.MethodGet, "/api/cleanup/preview", nil)
}

func (c *Client) RunCleanup(ctx context.Context) (UpdateCleanupResult, error) {
	return doJSON[UpdateCleanupResult](ctx, c, http.MethodPost, "/api/cleanup", nil)
}

// rawRequest issues a raw, non-JSON-decoded POST against Core with the
// long timeout backup export/restore need - shared by ExportBackup and
// RestoreBackupRequest, which just supply the path, content type, and body.
func (c *Client) rawRequest(ctx context.Context, path, contentType string, body io.Reader) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+c.token)
	request.Header.Set("Content-Type", contentType)
	client := *c.http
	client.Timeout = 10 * time.Minute
	return client.Do(request)
}

func (c *Client) ExportBackup(ctx context.Context, passphrase string) (*http.Response, error) {
	data, err := json.Marshal(map[string]string{"passphrase": passphrase})
	if err != nil {
		return nil, err
	}
	return c.rawRequest(ctx, "/api/backups/export", "application/json", bytes.NewReader(data))
}

func (c *Client) RestoreBackupRequest(ctx context.Context, path, contentType string, body io.Reader) (*http.Response, error) {
	return c.rawRequest(ctx, path, contentType, body)
}

func (c *Client) CheckUpdates(ctx context.Context) (UpdateStatus, error) {
	return doJSON[UpdateStatus](ctx, c, http.MethodPost, "/api/updates/check", nil)
}

func (c *Client) UpdateService(ctx context.Context, service string) (UpdateStatus, error) {
	return doJSON[UpdateStatus](ctx, c, http.MethodPost, "/api/updates/"+service, nil)
}

func (c *Client) ControlPlaneUpdateStatus(ctx context.Context) (ControlPlaneUpdateStatus, error) {
	return doJSON[ControlPlaneUpdateStatus](ctx, c, http.MethodGet, "/api/control-plane-updates", nil)
}

func (c *Client) CheckControlPlaneUpdates(ctx context.Context) (ControlPlaneUpdateStatus, error) {
	return doJSON[ControlPlaneUpdateStatus](ctx, c, http.MethodPost, "/api/control-plane-updates/check", nil)
}

func (c *Client) InstallControlPlaneUpdates(ctx context.Context) (ControlPlaneUpdateStatus, error) {
	return doJSON[ControlPlaneUpdateStatus](ctx, c, http.MethodPost, "/api/control-plane-updates/install", nil)
}

func (c *Client) UpdaterSelfUpdateStatus(ctx context.Context) (UpdateStatus, error) {
	return doJSON[UpdateStatus](ctx, c, http.MethodGet, "/api/updater-updates", nil)
}

func (c *Client) CheckUpdaterSelfUpdate(ctx context.Context) (UpdateStatus, error) {
	return doJSON[UpdateStatus](ctx, c, http.MethodPost, "/api/updater-updates/check", nil)
}

func (c *Client) InstallUpdaterSelfUpdate(ctx context.Context) (UpdateStatus, error) {
	return doJSON[UpdateStatus](ctx, c, http.MethodPost, "/api/updater-updates/install", nil)
}

func (c *Client) AttestationProxySelfUpdateStatus(ctx context.Context) (UpdateStatus, error) {
	return doJSON[UpdateStatus](ctx, c, http.MethodGet, "/api/attestation-proxy-updates", nil)
}

func (c *Client) CheckAttestationProxySelfUpdate(ctx context.Context) (UpdateStatus, error) {
	return doJSON[UpdateStatus](ctx, c, http.MethodPost, "/api/attestation-proxy-updates/check", nil)
}

func (c *Client) InstallAttestationProxySelfUpdate(ctx context.Context) (UpdateStatus, error) {
	return doJSON[UpdateStatus](ctx, c, http.MethodPost, "/api/attestation-proxy-updates/install", nil)
}

// doJSON is the shared shape behind every simple proxy method on Client:
// decode Core's JSON response into T, or propagate the error. Kept next to
// do, which it wraps.
func doJSON[T any](ctx context.Context, c *Client, method, path string, body any) (T, error) {
	var result T
	err := c.do(ctx, method, path, body, &result)
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
