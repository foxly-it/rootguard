// =====================================================
// File: frontend/src/api/client.ts
// Purpose: Central API client for RootGuard
// =====================================================

/**
 * Generic API helper
 */
async function request<T>(
  url: string,
  options?: RequestInit
): Promise<T> {
  const response = await fetch(url, {
    headers: {
      "Content-Type": "application/json",
    },
    ...options,
  });

  if (response.status === 401) {
    window.dispatchEvent(new Event("rootguard:unauthorized"));
  }

  if (!response.ok) {
    const detail = (await response.text()).trim();
    throw new Error(detail || `API Error: ${response.status}`);
  }

  return response.json();
}

// =====================================================
// Dashboard Endpoint
// =====================================================

export interface DashboardResponse {
  docker: {
    cpu: number;
    memory: number;
    metrics_available: boolean;
    containers: number;
    status: "healthy" | "degraded" | "down";
  };
  dns: {
    status: "healthy" | "degraded" | "down";
    resolver: string;
    dnssec: boolean;
  };
}

export async function fetchDashboard() {
  return request<DashboardResponse>("/api/dashboard");
}

export interface ServiceInfo {
  name: "core" | "webapp" | "updater" | "adguard" | "unbound";
  displayName: string;
  description: string;
  status: "running" | "stopped";
  health: "healthy" | "unhealthy" | "starting" | "not_configured" | "unknown";
  image?: string;
  imageId?: string;
  startedAt?: string;
  restartCount: number;
  ports?: string[];
  version?: string;
  revision?: string;
  created?: string;
  source?: string;
  immutable: boolean;
  metadata: "complete" | "partial" | "unavailable";
  attestation: "verified" | "missing" | "failed" | "unavailable" | "not_applicable";
  attestedAt?: string;
}

export async function fetchServices(): Promise<ServiceInfo[]> {
  return request<ServiceInfo[]>("/api/services");
}

export interface ServiceLogs {
  service: ServiceInfo["name"];
  lines: string[];
  tail: number;
  since: string;
  truncated: boolean;
  redacted: boolean;
  description: string;
}

export async function fetchServiceLogs(name: ServiceInfo["name"]): Promise<ServiceLogs> {
  return request<ServiceLogs>(`/api/services/${name}/logs`);
}

// =====================================================
// Service Action Endpoint
// =====================================================

export interface ServiceActionResponse {
  service: string;
  action: string;
  status: string;
}

export async function serviceAction(
  name: string,
  action: "start" | "stop" | "restart"
): Promise<void> {
  await request<ServiceActionResponse>(
    `/api/service/${name}/${action}`,
    {
      method: "POST",
    }
  );
}

// =====================================================
// Unbound Settings
// =====================================================

export interface UnboundSettings {
  qname_minimisation: boolean;
  prefetch: boolean;
  prefetch_key: boolean;
  aggressive_nsec: boolean;
  edns_buffer_size: number;
  log_verbosity: number;
  serve_expired: boolean;
  serve_expired_ttl: number;
  serve_expired_client_timeout: number;
  cache_min_ttl: number;
  cache_max_ttl: number;
  threads: number;
  resource_profile: "small" | "medium" | "large";
  network_mode: "ipv4" | "dual" | "ipv6";
  forward_zones: UnboundForwardZone[];
  private_domains: string[];
  reverse_zones: UnboundReverseZonePolicy[];
}

export interface UnboundForwardZone {
  name: string;
  servers: string[];
  forward_first: boolean;
  allow_unsigned: boolean;
  allow_private_addresses: boolean;
}

export interface UnboundReverseZonePolicy {
  network: "10.0.0.0/8" | "172.16.0.0/12" | "192.168.0.0/16";
  mode: "nxdomain" | "transparent";
}

export interface UnboundForwardTargetCheck {
  zone: string;
  address: string;
  reachable: boolean;
  detail: string;
}

export interface UnboundNetworkCapabilities {
  ipv4_available: boolean;
  ipv4_detail: string;
  ipv6_available: boolean;
  ipv6_detail: string;
  checked_at: string;
}

export async function fetchUnboundSettings(): Promise<UnboundSettings> {
  return request<UnboundSettings>("/api/unbound/settings");
}

export interface UnboundActiveConfiguration {
  base_config: string;
  managed_config: string;
  custom_config: string;
  checked_at: string;
}

export async function fetchUnboundActiveConfiguration(): Promise<UnboundActiveConfiguration> {
  return request<UnboundActiveConfiguration>("/api/unbound/config");
}

export async function updateUnboundSettings(
  settings: UnboundSettings
): Promise<UnboundSettings> {
  return request<UnboundSettings>("/api/unbound/settings", {
    method: "PUT",
    body: JSON.stringify(settings),
  });
}

export interface UnboundChange {
  field: string;
  before: string;
  after: string;
}

export interface UnboundPreview {
  changed: boolean;
  changes: UnboundChange[];
  rendered_config: string;
}

export interface UnboundHistoryEntry {
  id: string;
  created_at: string;
  settings: UnboundSettings;
  config?: string;
  custom_config?: string;
}

export interface UnboundDiagnosticCheck {
  name: string;
  passed: boolean;
  detail: string;
}

export interface UnboundDiagnosticReport {
  healthy: boolean;
  checked_at: string;
  checks: UnboundDiagnosticCheck[];
}

export interface UnboundPreset {
  id: string;
  name: string;
  description: string;
  best_for: string;
  settings: UnboundSettings;
}

export interface UnboundRecommendation {
  id: string;
  severity: "success" | "recommendation" | "warning";
  field?: string;
  title: string;
  description: string;
  suggestion: string;
}

export interface UnboundAdvice {
  status: "optimized" | "suggestions" | "review";
  recommendations: UnboundRecommendation[];
}

export async function previewUnboundSettings(settings: UnboundSettings): Promise<UnboundPreview> {
  return request<UnboundPreview>("/api/unbound/preview", {
    method: "POST",
    body: JSON.stringify(settings),
  });
}

export async function fetchUnboundHistory(): Promise<UnboundHistoryEntry[]> {
  return request<UnboundHistoryEntry[]>("/api/unbound/history");
}

export async function restoreUnboundVersion(id: string): Promise<UnboundSettings> {
  return request<UnboundSettings>(`/api/unbound/history/${encodeURIComponent(id)}/restore`, {
    method: "POST",
  });
}

export async function fetchUnboundDiagnostics(): Promise<UnboundDiagnosticReport> {
  return request<UnboundDiagnosticReport>("/api/unbound/diagnostics");
}

export interface UnboundDiagnosticLoggingStatus {
  active: boolean;
  expires_at?: string;
  level: number;
}

export async function fetchUnboundDiagnosticLoggingStatus(): Promise<UnboundDiagnosticLoggingStatus> {
  return request<UnboundDiagnosticLoggingStatus>("/api/unbound/diagnostic-logging");
}

export async function startUnboundDiagnosticLogging(): Promise<UnboundDiagnosticLoggingStatus> {
  return request<UnboundDiagnosticLoggingStatus>("/api/unbound/diagnostic-logging", { method: "POST" });
}

export async function stopUnboundDiagnosticLogging(): Promise<UnboundDiagnosticLoggingStatus> {
  return request<UnboundDiagnosticLoggingStatus>("/api/unbound/diagnostic-logging", { method: "DELETE" });
}

export async function fetchUnboundPresets(): Promise<UnboundPreset[]> {
  return request<UnboundPreset[]>("/api/unbound/presets");
}

export async function fetchUnboundAdvice(settings: UnboundSettings): Promise<UnboundAdvice> {
  return request<UnboundAdvice>("/api/unbound/advice", {
    method: "POST",
    body: JSON.stringify(settings),
  });
}

export async function checkUnboundForwardTargets(zones: UnboundForwardZone[]): Promise<UnboundForwardTargetCheck[]> {
  return request<UnboundForwardTargetCheck[]>("/api/unbound/forward-check", {
    method: "POST",
    body: JSON.stringify({ zones }),
  });
}

export async function fetchUnboundNetworkCapabilities(): Promise<UnboundNetworkCapabilities> {
  return request<UnboundNetworkCapabilities>("/api/unbound/network-capabilities");
}

export interface UnboundCustomDocument {
  content: string;
  max_bytes: number;
}

export interface UnboundCustomAdvice {
  id: string;
  severity: "success" | "recommendation" | "warning";
  line?: number;
  title: string;
  description: string;
  suggestion: string;
}

export interface UnboundCustomPreview {
  changed: boolean;
  content: string;
  validation: string;
  advice: UnboundCustomAdvice[];
}

export interface UnboundDirectiveReference {
  name: string;
  section: string;
  example: string;
  description: string;
  risk: "low" | "medium" | "high";
}

export async function fetchUnboundCustom(): Promise<UnboundCustomDocument> {
  return request<UnboundCustomDocument>("/api/unbound/custom");
}

export async function previewUnboundCustom(content: string): Promise<UnboundCustomPreview> {
  return request<UnboundCustomPreview>("/api/unbound/custom/preview", {
    method: "POST",
    body: JSON.stringify({ content }),
  });
}

export async function updateUnboundCustom(content: string): Promise<UnboundCustomDocument> {
  return request<UnboundCustomDocument>("/api/unbound/custom", {
    method: "PUT",
    body: JSON.stringify({ content }),
  });
}

export async function fetchUnboundDirectives(): Promise<UnboundDirectiveReference[]> {
  return request<UnboundDirectiveReference[]>("/api/unbound/directives");
}

export interface AdGuardStatus {
  configured: boolean;
  healthy: boolean;
  upstream: string;
  upstream_ready: boolean;
  stats_available: boolean;
  queries: number;
  blocked: number;
  average_response_seconds: number;
  best_practices_ready: boolean;
}

export async function fetchAdGuardStatus(): Promise<AdGuardStatus> {
  return request<AdGuardStatus>("/api/adguard/status");
}

export async function bootstrapAdGuard(): Promise<AdGuardStatus> {
  return request<AdGuardStatus>("/api/adguard/bootstrap", { method: "POST" });
}

export interface AdGuardFilterCheck {
  host: string;
  category: "advertising" | "tracking" | "service" | "telemetry" | "security-test";
  expected_blocked: boolean;
  blocked: boolean;
  reason: string;
  matched_rule?: string;
}

export interface AdGuardFilterReport {
  checks: AdGuardFilterCheck[];
  blocked: number;
  expected: number;
  passed: number;
  checked_at: string;
}

export async function fetchAdGuardFilterReport(): Promise<AdGuardFilterReport> {
  return request<AdGuardFilterReport>("/api/adguard/filter-report");
}

// =====================================================
// AIO installation
// =====================================================

export interface InstallationConfig {
  dns_bind_address: string;
  dns_port: number;
  adguard_channel: "stable" | "beta";
}

export interface InstallationCheck {
  id: string;
  code: string;
  ok: boolean;
  message: string;
  detail?: string;
  action?: string;
}

export interface InstallationPreflight {
  ready: boolean;
  config: InstallationConfig;
  checks: InstallationCheck[];
}

export interface InstallationStep {
  id: string;
  status: "pending" | "running" | "done" | "failed";
  message: string;
}

export interface InstallationStatus {
  state: "not_installed" | "deploying" | "installed" | "failed";
  config?: InstallationConfig;
  steps: InstallationStep[];
  error?: string;
  diagnostic?: InstallationDiagnostic;
  updated_at: string;
}

export interface InstallationDiagnostic {
  code: string;
  phase: string;
  message: string;
  detail?: string;
  action: string;
  retryable: boolean;
}

export async function fetchInstallationStatus(): Promise<InstallationStatus> {
  return request<InstallationStatus>("/api/installation");
}

export async function preflightInstallation(
  config: InstallationConfig
): Promise<InstallationPreflight> {
  return request<InstallationPreflight>("/api/installation/preflight", {
    method: "POST",
    body: JSON.stringify(config),
  });
}

export async function deployInstallation(
  config: InstallationConfig
): Promise<InstallationStatus> {
  return request<InstallationStatus>("/api/installation/deploy", {
    method: "POST",
    body: JSON.stringify(config),
  });
}

// =====================================================
// Stack updates
// =====================================================

export interface UpdateServiceStatus {
  name: "adguard" | "unbound";
  display_name: string;
  current_image?: string;
  target_image: string;
  current_id?: string;
  candidate_id?: string;
  update_available: boolean;
  checked_at?: string;
  error?: string;
}

export interface UpdateCleanupResult {
  removed_images?: string[];
  removed_volumes?: string[];
  skipped?: string[];
}

export interface UpdateHistoryEntry {
  service?: string;
  outcome: "success" | "rolled_back" | "failed" | "no_change";
  from_id?: string;
  to_id?: string;
  from_ids?: Record<string, string>;
  to_ids?: Record<string, string>;
  message: string;
  cleanup: UpdateCleanupResult;
  created_at: string;
}

export interface UpdateStatus {
  state: "idle" | "checking" | "updating" | "failed";
  active_service?: string;
  message: string;
  services: UpdateServiceStatus[];
  history?: UpdateHistoryEntry[];
  updated_at: string;
}

export async function fetchUpdateStatus(): Promise<UpdateStatus> {
  return request<UpdateStatus>("/api/updates");
}

export async function checkUpdates(): Promise<UpdateStatus> {
  return request<UpdateStatus>("/api/updates/check", { method: "POST" });
}

export async function installServiceUpdate(service: "adguard" | "unbound"): Promise<UpdateStatus> {
  return request<UpdateStatus>(`/api/updates/${service}`, { method: "POST" });
}

export interface ControlPlaneUpdateServiceStatus extends Omit<UpdateServiceStatus, "name"> {
  name: "core" | "webapp";
}

export interface ControlPlaneUpdateStatus {
  state: "idle" | "checking" | "updating" | "failed";
  message: string;
  services: ControlPlaneUpdateServiceStatus[];
  history?: UpdateHistoryEntry[];
  updated_at: string;
}

export async function fetchControlPlaneUpdateStatus(): Promise<ControlPlaneUpdateStatus> {
  return request<ControlPlaneUpdateStatus>("/api/control-plane-updates");
}

export async function checkControlPlaneUpdates(): Promise<ControlPlaneUpdateStatus> {
  return request<ControlPlaneUpdateStatus>("/api/control-plane-updates/check", { method: "POST" });
}

export async function installControlPlaneUpdates(): Promise<ControlPlaneUpdateStatus> {
  return request<ControlPlaneUpdateStatus>("/api/control-plane-updates/install", { method: "POST" });
}
