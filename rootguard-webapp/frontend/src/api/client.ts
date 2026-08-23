// =====================================================
// File: frontend/src/api/client.ts
// Purpose: Central API client for RootGuard
// =====================================================

/**
 * Generic API helper - returns the raw Response, e.g. for a binary body or
 * response headers that request<T> below can't expose (JSON-only).
 */
async function requestRaw(url: string, options?: RequestInit): Promise<Response> {
  // A FormData body must not get an explicit Content-Type: the browser sets
  // its own multipart boundary automatically, and overriding it breaks the
  // upload.
  const defaultHeaders =
    options?.body instanceof FormData ? undefined : { "Content-Type": "application/json" };
  const response = await fetch(url, {
    ...options,
    headers: { ...defaultHeaders, ...options?.headers },
  });

  if (response.status === 401) {
    window.dispatchEvent(new Event("rootguard:unauthorized"));
  }

  if (!response.ok) {
    const detail = (await response.text()).trim();
    throw new Error(detail || `API Error: ${response.status}`);
  }

  return response;
}

/**
 * Generic API helper
 */
async function request<T>(url: string, options?: RequestInit): Promise<T> {
  return (await requestRaw(url, options)).json();
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

// =====================================================
// Session Inventory
// =====================================================

export interface SessionSummary {
  id: string;
  username: string;
  created_at: string;
  expires_at: string;
  user_agent: string;
  remote_ip: string;
  current: boolean;
}

export async function fetchSessions(): Promise<SessionSummary[]> {
  return request<SessionSummary[]>("/api/auth/sessions");
}

export async function revokeSession(id: string): Promise<void> {
  await request<{ revoked: boolean }>(`/api/auth/sessions/${encodeURIComponent(id)}`, { method: "DELETE" });
}

export interface AccountUpdateResult {
  updated: boolean;
  username: string;
}

export async function updateAccount(input: { current_password: string; new_username?: string; new_password?: string }): Promise<AccountUpdateResult> {
  return request<AccountUpdateResult>("/api/auth/account", { method: "POST", body: JSON.stringify(input) });
}

export interface AuditEvent {
  timestamp: string;
  event: "login_success" | "login_failure" | "login_rate_limited" | "logout" | "recovery_success" | "recovery_failure" | "session_revoked" | "account_updated" | "account_update_failure";
  username?: string;
  remote_ip: string;
}

export async function fetchAuditLog(): Promise<AuditEvent[]> {
  return request<AuditEvent[]>("/api/auth/audit");
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
  local_zones: UnboundLocalZone[];
}

export interface UnboundLocalHost {
  hostname: string;
  ipv4?: string;
  ipv6?: string;
  ptr: boolean;
}

export interface UnboundLocalZone {
  name: string;
  hosts: UnboundLocalHost[];
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

export interface UnboundConfigBundle {
  schema_version: number;
  exported_at: string;
  settings: UnboundSettings;
  custom_config: string;
}

export interface UnboundBundlePreview {
  changed: boolean;
  changes: UnboundChange[];
  custom_changed: boolean;
  rendered_config: string;
}

export async function fetchUnboundExport(): Promise<UnboundConfigBundle> {
  return request<UnboundConfigBundle>("/api/unbound/export");
}

export async function previewUnboundImport(bundle: UnboundConfigBundle): Promise<UnboundBundlePreview> {
  return request<UnboundBundlePreview>("/api/unbound/import/preview", {
    method: "POST",
    body: JSON.stringify(bundle),
  });
}

export async function applyUnboundImport(bundle: UnboundConfigBundle): Promise<UnboundSettings> {
  return request<UnboundSettings>("/api/unbound/import", {
    method: "POST",
    body: JSON.stringify(bundle),
  });
}

export type UnboundImportDisposition = "guided" | "fixed_base" | "expert" | "blocked";

export interface UnboundImportFinding {
  section: string;
  line: number;
  directive: string;
  value?: string;
  disposition: UnboundImportDisposition;
  detail: string;
}

export interface UnboundImportResult {
  findings: UnboundImportFinding[];
  settings: UnboundSettings;
  custom_adopted: string;
}

export async function classifyUnboundImportConf(content: string): Promise<UnboundImportResult> {
  return request<UnboundImportResult>("/api/unbound/import-conf", {
    method: "POST",
    body: JSON.stringify({ content }),
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

export async function fetchUnboundPathDiagnostics(): Promise<UnboundDiagnosticReport> {
  return request<UnboundDiagnosticReport>("/api/unbound/path-diagnostics");
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

export interface DiscoveredHost {
  hostname: string;
  ipv4?: string;
  ipv6?: string;
  mac?: string;
  active: boolean;
  source: string;
}

export interface RouterDiscoveryResult {
  hosts: DiscoveredHost[];
  truncated: boolean;
  scanned?: number;
  failed?: number;
}

export async function discoverReverseDNSHosts(networks: string[]): Promise<RouterDiscoveryResult> {
  return request<RouterDiscoveryResult>("/api/router-import/reverse-dns/discover", {
    method: "POST",
    body: JSON.stringify({ networks }),
  });
}

export async function discoverFritzBoxHosts(address: string, username: string, password: string): Promise<RouterDiscoveryResult> {
  return request<RouterDiscoveryResult>("/api/router-import/fritzbox/discover", {
    method: "POST",
    body: JSON.stringify({ address, username, password }),
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
  version?: string;
  upstream: string;
  upstream_ready: boolean;
  stats_available: boolean;
  queries: number;
  blocked: number;
  average_response_seconds: number;
  best_practices_ready: boolean;
  filtering_enabled: boolean;
  active_filter_lists: number;
  total_filter_lists: number;
}

export async function fetchAdGuardStatus(): Promise<AdGuardStatus> {
  return request<AdGuardStatus>("/api/adguard/status");
}

export async function bootstrapAdGuard(): Promise<AdGuardStatus> {
  return request<AdGuardStatus>("/api/adguard/bootstrap", { method: "POST" });
}

export async function setAdGuardFiltering(enabled: boolean): Promise<AdGuardStatus> {
  return request<AdGuardStatus>("/api/adguard/filtering", {
    method: "POST",
    body: JSON.stringify({ enabled }),
  });
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
  blockpage_enabled: boolean;
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
  outcome: "success" | "rolled_back" | "failed" | "no_change" | "cleanup";
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

export interface BackupServiceUsage {
  service: "adguard" | "unbound";
  count: number;
  bytes: number;
  oldest_at?: string;
  newest_at?: string;
}

export interface BackupStatus {
  settings: { retention_per_service: number };
  count: number;
  managed_bytes: number;
  unmanaged_bytes: number;
  services: BackupServiceUsage[];
  last_error?: string;
}

export async function fetchBackupStatus(): Promise<BackupStatus> {
  return request<BackupStatus>("/api/backups");
}

export async function setBackupRetention(retentionPerService: number): Promise<BackupStatus> {
  return request<BackupStatus>("/api/backups/settings", {
    method: "PUT",
    body: JSON.stringify({ retention_per_service: retentionPerService }),
  });
}

export interface CleanupResource {
  kind: "image" | "volume";
  id: string;
  estimated_bytes: number;
}

export interface CleanupPreview {
  resources: CleanupResource[];
  skipped?: string[];
  estimated_bytes: number;
}

export async function fetchCleanupPreview(): Promise<CleanupPreview> {
  return request<CleanupPreview>("/api/cleanup/preview");
}

export async function runManualCleanup(): Promise<UpdateCleanupResult> {
  return request<UpdateCleanupResult>("/api/cleanup", { method: "POST" });
}

export async function exportEncryptedBackup(passphrase: string): Promise<{ blob: Blob; filename: string }> {
  const response = await requestRaw("/api/backups/export", {
    method: "POST",
    body: JSON.stringify({ passphrase }),
  });
  const disposition = response.headers.get("Content-Disposition") ?? "";
  const filename = disposition.match(/filename="([^"]+)"/)?.[1] ?? "rootguard-backup.tar.gz.age";
  return { blob: await response.blob(), filename };
}

export interface BackupRestorePreview {
  schema_version: number;
  created_at: string;
  file_count: number;
  expanded_bytes: number;
  config: InstallationConfig;
  preflight: InstallationPreflight;
}

function restoreForm(file: File, passphrase: string, config?: InstallationConfig, confirmation?: string) {
  const form = new FormData();
  form.append("passphrase", passphrase);
  if (config) form.append("config", JSON.stringify(config));
  if (confirmation) form.append("confirmation", confirmation);
  form.append("archive", file);
  return form;
}

export async function previewEncryptedBackup(file: File, passphrase: string, config?: InstallationConfig): Promise<BackupRestorePreview> {
  return request<BackupRestorePreview>("/api/backups/restore/preview", {
    method: "POST",
    body: restoreForm(file, passphrase, config),
  });
}

export async function restoreEncryptedBackup(file: File, passphrase: string, config: InstallationConfig): Promise<InstallationStatus> {
  return request<InstallationStatus>("/api/backups/restore", {
    method: "POST",
    body: restoreForm(file, passphrase, config, "RESTORE"),
  });
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

export interface UpdaterSelfUpdateServiceStatus extends Omit<UpdateServiceStatus, "name"> {
  name: "updater";
}

export interface UpdaterSelfUpdateStatus {
  state: "idle" | "checking" | "updating" | "failed";
  active_service?: string;
  message: string;
  services: UpdaterSelfUpdateServiceStatus[];
  history?: UpdateHistoryEntry[];
  updated_at: string;
}

export async function fetchUpdaterSelfUpdateStatus(): Promise<UpdaterSelfUpdateStatus> {
  return request<UpdaterSelfUpdateStatus>("/api/updater-updates");
}

export async function checkUpdaterSelfUpdate(): Promise<UpdaterSelfUpdateStatus> {
  return request<UpdaterSelfUpdateStatus>("/api/updater-updates/check", { method: "POST" });
}

export async function installUpdaterSelfUpdate(): Promise<UpdaterSelfUpdateStatus> {
  return request<UpdaterSelfUpdateStatus>("/api/updater-updates/install", { method: "POST" });
}
