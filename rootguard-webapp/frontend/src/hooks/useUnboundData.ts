import { useCallback, useEffect, useState, type FormEvent } from "react";
import {
  fetchUnboundDiagnostics,
  fetchUnboundPathDiagnostics,
  fetchUnboundDiagnosticLoggingStatus,
  fetchUnboundHistory,
  fetchUnboundAdvice,
  fetchUnboundActiveConfiguration,
  fetchUnboundPresets,
  fetchUnboundSettings,
  fetchUnboundNetworkCapabilities,
  previewUnboundSettings,
  startUnboundDiagnosticLogging,
  stopUnboundDiagnosticLogging,
  restoreUnboundVersion,
  updateUnboundSettings,
  type UnboundDiagnosticReport,
  type UnboundDiagnosticLoggingStatus,
  type UnboundAdvice,
  type UnboundActiveConfiguration,
  type UnboundHistoryEntry,
  type UnboundPreset,
  type UnboundPreview,
  type UnboundSettings,
  type UnboundNetworkCapabilities,
} from "../api/client";
import { useI18n } from "../i18n";
import { useInterval } from "./useInterval";
import { errorMessage } from "../utils/errors";
import { presetText } from "../utils/unboundText";

/**
 * All of Unbound.tsx's data-fetching, polling, and mutation logic -
 * extracted out of the page component (found in review: data logic and
 * JSX were mixed into one large component) so the page itself is just a
 * presentation layer over this hook's return value. Routing (active
 * tab/section) and purely presentational UI state (which read-only config
 * modal is open) stay in the page component - they're not data this hook
 * owns.
 */
export function useUnboundData() {
  const { t, formatDate } = useI18n();
  const [settings, setSettings] = useState<UnboundSettings | null>(null);
  const [history, setHistory] = useState<UnboundHistoryEntry[]>([]);
  const [preview, setPreview] = useState<UnboundPreview | null>(null);
  const [diagnostics, setDiagnostics] = useState<UnboundDiagnosticReport | null>(null);
  const [pathDiagnostics, setPathDiagnostics] = useState<UnboundDiagnosticReport | null>(null);
  const [diagnosticLogging, setDiagnosticLogging] = useState<UnboundDiagnosticLoggingStatus | null>(null);
  const [presets, setPresets] = useState<UnboundPreset[]>([]);
  const [advice, setAdvice] = useState<UnboundAdvice | null>(null);
  const [liveConfig, setLiveConfig] = useState<UnboundActiveConfiguration | null>(null);
  const [networkCapabilities, setNetworkCapabilities] = useState<UnboundNetworkCapabilities | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  const reload = useCallback(async () => {
    const [loadedSettings, loadedHistory, loadedPresets, loadedConfig, loadedDiagnosticLogging] = await Promise.all([
      fetchUnboundSettings(),
      fetchUnboundHistory(),
      fetchUnboundPresets(),
      fetchUnboundActiveConfiguration(),
      fetchUnboundDiagnosticLoggingStatus(),
    ]);
    setSettings({
      ...loadedSettings,
      forward_zones: loadedSettings.forward_zones ?? [],
      private_domains: loadedSettings.private_domains ?? [],
      reverse_zones: loadedSettings.reverse_zones ?? [],
      local_zones: loadedSettings.local_zones ?? [],
      network_mode: loadedSettings.network_mode ?? "ipv4",
      resource_profile: loadedSettings.resource_profile ?? "medium",
      prefetch_key: loadedSettings.prefetch_key ?? true,
      aggressive_nsec: loadedSettings.aggressive_nsec ?? true,
      edns_buffer_size: loadedSettings.edns_buffer_size ?? 1232,
      log_verbosity: loadedSettings.log_verbosity ?? 1,
      serve_expired_ttl: loadedSettings.serve_expired_ttl ?? 86400,
      serve_expired_client_timeout: loadedSettings.serve_expired_client_timeout ?? 1800,
    });
    setHistory(loadedHistory);
    setPresets(loadedPresets);
    setLiveConfig(loadedConfig);
    setDiagnosticLogging(loadedDiagnosticLogging);
  }, []);

  function clearFeedback() {
    setMessage("");
    setError("");
  }

  async function withBusy(action: () => Promise<void>, fallback: string, guard?: () => boolean) {
    if (busy || (guard && !guard())) return;
    setBusy(true);
    clearFeedback();
    try {
      await action();
    } catch (err) {
      setError(errorMessage(err, t(fallback)));
    } finally {
      setBusy(false);
    }
  }

  function checkNetworkCapabilities() {
    return withBusy(async () => {
      setNetworkCapabilities(await fetchUnboundNetworkCapabilities());
      setMessage(t("network.checked"));
    }, "network.checkError");
  }

  useEffect(() => {
    reload()
      .catch((err: unknown) => setError(errorMessage(err, t("unbound.loadError"))))
      .finally(() => setLoading(false));
  }, [reload, t]);

  useEffect(() => {
    if (!settings) return;
    let current = true;
    const request = window.setTimeout(() => {
      fetchUnboundAdvice(settings)
        .then((nextAdvice) => { if (current) setAdvice(nextAdvice); })
        .catch(() => { if (current) setAdvice(null); });
    }, 250);
    return () => {
      current = false;
      window.clearTimeout(request);
    };
  }, [settings]);

  useInterval(() => {
    fetchUnboundDiagnosticLoggingStatus()
      .then(setDiagnosticLogging)
      .catch(() => undefined);
  }, diagnosticLogging?.active ? 10_000 : null);

  function selectPreset(preset: UnboundPreset) {
    return withBusy(async () => {
      if (!settings) return;
      const proposed = {
        ...preset.settings,
        forward_zones: settings.forward_zones,
        private_domains: settings.private_domains,
        reverse_zones: settings.reverse_zones,
        local_zones: settings.local_zones,
        network_mode: settings.network_mode,
      };
      setSettings(proposed);
      setPreview(await previewUnboundSettings(proposed));
      setMessage(t("unbound.presetLoaded", { name: presetText(preset.id, "name", t, preset.name) }));
    }, "unbound.presetError", () => !!settings);
  }

  function createPreview(event: FormEvent) {
    event.preventDefault();
    return withBusy(async () => {
      if (!settings) return;
      setPreview(await previewUnboundSettings(settings));
    }, "unbound.previewError", () => !!settings);
  }

  function applyPreview() {
    return withBusy(async () => {
      if (!settings) return;
      const updated = await updateUnboundSettings(settings);
      setSettings(updated);
      setPreview(null);
      await reload();
      setMessage(t("unbound.activated"));
    }, "unbound.activateError", () => !!settings && !!preview?.changed);
  }

  function restore(entry: UnboundHistoryEntry) {
    return withBusy(async () => {
      setSettings(await restoreUnboundVersion(entry.id));
      setPreview(null);
      await reload();
      setMessage(t("unbound.restored"));
    }, "unbound.restoreError", () => window.confirm(t("unbound.confirmRestore", { date: formatDate(entry.created_at) })));
  }

  function runDiagnostics() {
    return withBusy(async () => {
      setDiagnostics(await fetchUnboundDiagnostics());
    }, "unbound.diagnosticError");
  }

  function runPathDiagnostics() {
    return withBusy(async () => {
      setPathDiagnostics(await fetchUnboundPathDiagnostics());
    }, "unbound.pathDiagnosticError");
  }

  function toggleDiagnosticLogging() {
    return withBusy(async () => {
      const status = diagnosticLogging?.active
        ? await stopUnboundDiagnosticLogging()
        : await startUnboundDiagnosticLogging();
      setDiagnosticLogging(status);
      setMessage(t(status.active ? "unbound.diagnosticLoggingStarted" : "unbound.diagnosticLoggingStopped"));
    }, "unbound.diagnosticLoggingError");
  }

  return {
    settings,
    setSettings,
    history,
    preview,
    setPreview,
    diagnostics,
    pathDiagnostics,
    diagnosticLogging,
    presets,
    advice,
    liveConfig,
    networkCapabilities,
    loading,
    busy,
    message,
    error,
    reload,
    checkNetworkCapabilities,
    selectPreset,
    createPreview,
    applyPreview,
    restore,
    runDiagnostics,
    runPathDiagnostics,
    toggleDiagnosticLogging,
  };
}
