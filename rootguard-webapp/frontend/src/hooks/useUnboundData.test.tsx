import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { I18nProvider } from "../i18n/provider";
import * as client from "../api/client";
import { useUnboundData } from "./useUnboundData";

const baseSettings: client.UnboundSettings = {
  qname_minimisation: true,
  prefetch: true,
  prefetch_key: true,
  aggressive_nsec: true,
  edns_buffer_size: 1232,
  log_verbosity: 1,
  serve_expired: true,
  serve_expired_ttl: 86400,
  serve_expired_client_timeout: 1800,
  cache_min_ttl: 0,
  cache_max_ttl: 86400,
  threads: 2,
  resource_profile: "medium",
  network_mode: "ipv4",
  forward_zones: [{ name: "example.", servers: ["172.30.54.2"], forward_first: false, allow_unsigned: false, allow_private_addresses: false }],
  private_domains: ["home.example."],
  reverse_zones: [],
  local_zones: [],
};

const baseHistoryEntry: client.UnboundHistoryEntry = {
  id: "v1",
  created_at: new Date().toISOString(),
  settings: baseSettings,
};

const baseLiveConfig: client.UnboundActiveConfiguration = {
  base_config: "server:\n",
  managed_config: "",
  custom_config: "",
  checked_at: new Date().toISOString(),
};

const idleDiagnosticLogging: client.UnboundDiagnosticLoggingStatus = { active: false, level: 0 };
const activeDiagnosticLogging: client.UnboundDiagnosticLoggingStatus = { active: true, level: 2, expires_at: new Date().toISOString() };

const preset: client.UnboundPreset = {
  id: "balanced",
  name: "Balanced",
  description: "",
  best_for: "",
  settings: { ...baseSettings, threads: 4, forward_zones: [], private_domains: [], reverse_zones: [], local_zones: [] },
};

function wrapper({ children }: { children: ReactNode }) {
  return <I18nProvider>{children}</I18nProvider>;
}

async function flush() {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(0);
  });
}

async function advance(ms: number) {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(ms);
  });
}

beforeEach(() => {
  vi.restoreAllMocks();
  vi.useFakeTimers();
  vi.spyOn(client, "fetchUnboundSettings").mockResolvedValue(baseSettings);
  vi.spyOn(client, "fetchUnboundHistory").mockResolvedValue([baseHistoryEntry]);
  vi.spyOn(client, "fetchUnboundPresets").mockResolvedValue([preset]);
  vi.spyOn(client, "fetchUnboundActiveConfiguration").mockResolvedValue(baseLiveConfig);
  vi.spyOn(client, "fetchUnboundDiagnosticLoggingStatus").mockResolvedValue(idleDiagnosticLogging);
  vi.spyOn(client, "fetchUnboundAdvice").mockResolvedValue({ status: "optimized", recommendations: [] });
  vi.spyOn(client, "fetchUnboundNetworkCapabilities").mockResolvedValue({ ipv4_available: true, ipv4_detail: "", ipv6_available: false, ipv6_detail: "", checked_at: new Date().toISOString() });
  vi.spyOn(client, "previewUnboundSettings").mockImplementation(async (settings) => ({ changed: true, changes: [], rendered_config: JSON.stringify(settings) }));
  vi.spyOn(client, "updateUnboundSettings").mockImplementation(async (settings) => settings);
  vi.spyOn(client, "restoreUnboundVersion").mockResolvedValue(baseSettings);
  vi.spyOn(client, "startUnboundDiagnosticLogging").mockResolvedValue(activeDiagnosticLogging);
  vi.spyOn(client, "stopUnboundDiagnosticLogging").mockResolvedValue(idleDiagnosticLogging);
  vi.spyOn(client, "fetchUnboundDiagnostics").mockResolvedValue({ healthy: true, checked_at: new Date().toISOString(), checks: [] });
  vi.spyOn(client, "fetchUnboundPathDiagnostics").mockResolvedValue({ healthy: true, checked_at: new Date().toISOString(), checks: [] });
});

afterEach(() => {
  vi.useRealTimers();
});

describe("useUnboundData", () => {
  it("loads settings/history/presets/live config on mount", async () => {
    const { result } = renderHook(() => useUnboundData(), { wrapper });
    await flush();
    expect(result.current.loading).toBe(false);
    expect(result.current.settings?.threads).toBe(2);
    expect(result.current.history).toEqual([baseHistoryEntry]);
    expect(result.current.presets).toEqual([preset]);
    expect(result.current.liveConfig).toEqual(baseLiveConfig);
  });

  it("fills in defaults for settings fields the backend might omit", async () => {
    vi.spyOn(client, "fetchUnboundSettings").mockResolvedValue({
      ...baseSettings,
      forward_zones: undefined as unknown as client.UnboundForwardZone[],
      resource_profile: undefined as unknown as client.UnboundSettings["resource_profile"],
    });
    const { result } = renderHook(() => useUnboundData(), { wrapper });
    await flush();
    expect(result.current.settings?.forward_zones).toEqual([]);
    expect(result.current.settings?.resource_profile).toBe("medium");
  });

  it("fetches advice 250ms after settings change, not immediately", async () => {
    renderHook(() => useUnboundData(), { wrapper });
    await flush();
    expect(client.fetchUnboundAdvice).not.toHaveBeenCalled();
    await advance(250);
    expect(client.fetchUnboundAdvice).toHaveBeenCalledTimes(1);
  });

  it("only polls diagnostic-logging status once it is actually active", async () => {
    const { result } = renderHook(() => useUnboundData(), { wrapper });
    await flush();
    const callsWhileIdle = vi.mocked(client.fetchUnboundDiagnosticLoggingStatus).mock.calls.length;
    await advance(30_000);
    expect(vi.mocked(client.fetchUnboundDiagnosticLoggingStatus).mock.calls.length).toBe(callsWhileIdle);

    await act(async () => {
      await result.current.toggleDiagnosticLogging();
    });
    const callsOnceActive = vi.mocked(client.fetchUnboundDiagnosticLoggingStatus).mock.calls.length;
    await advance(10_000);
    expect(vi.mocked(client.fetchUnboundDiagnosticLoggingStatus).mock.calls.length).toBeGreaterThan(callsOnceActive);
  });

  it("toggleDiagnosticLogging starts logging when idle and stops it when active", async () => {
    const { result } = renderHook(() => useUnboundData(), { wrapper });
    await flush();

    await act(async () => {
      await result.current.toggleDiagnosticLogging();
    });
    expect(client.startUnboundDiagnosticLogging).toHaveBeenCalledTimes(1);
    expect(result.current.diagnosticLogging?.active).toBe(true);

    await act(async () => {
      await result.current.toggleDiagnosticLogging();
    });
    expect(client.stopUnboundDiagnosticLogging).toHaveBeenCalledTimes(1);
    expect(result.current.diagnosticLogging?.active).toBe(false);
  });

  it("selectPreset keeps the current zone/network fields instead of the preset's own (empty) ones", async () => {
    const { result } = renderHook(() => useUnboundData(), { wrapper });
    await flush();

    await act(async () => {
      await result.current.selectPreset(preset);
    });

    expect(result.current.settings?.threads).toBe(4); // from the preset
    expect(result.current.settings?.forward_zones).toEqual(baseSettings.forward_zones); // kept from current settings
    expect(result.current.settings?.private_domains).toEqual(baseSettings.private_domains);
    expect(result.current.preview?.changed).toBe(true);
  });

  it("restore does nothing when the confirm dialog is declined", async () => {
    vi.spyOn(window, "confirm").mockReturnValue(false);
    const { result } = renderHook(() => useUnboundData(), { wrapper });
    await flush();

    await act(async () => {
      await result.current.restore(baseHistoryEntry);
    });

    expect(client.restoreUnboundVersion).not.toHaveBeenCalled();
  });

  it("restore reloads settings once confirmed", async () => {
    vi.spyOn(window, "confirm").mockReturnValue(true);
    const { result } = renderHook(() => useUnboundData(), { wrapper });
    await flush();

    await act(async () => {
      await result.current.restore(baseHistoryEntry);
    });

    expect(client.restoreUnboundVersion).toHaveBeenCalledWith(baseHistoryEntry.id);
    expect(result.current.message).not.toBe("");
  });
});
