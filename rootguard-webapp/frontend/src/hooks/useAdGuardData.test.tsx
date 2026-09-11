import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { I18nProvider } from "../i18n/provider";
import * as client from "../api/client";
import { useAdGuardData } from "./useAdGuardData";

const installed: client.InstallationStatus = {
  state: "installed",
  steps: [],
  updated_at: new Date().toISOString(),
};

const notInstalled: client.InstallationStatus = {
  state: "not_installed",
  steps: [],
  updated_at: new Date().toISOString(),
};

const baseStatus: client.AdGuardStatus = {
  configured: true,
  healthy: true,
  upstream: "172.29.53.2:5335",
  upstream_ready: true,
  stats_available: true,
  queries: 100,
  blocked: 10,
  average_response_seconds: 0.02,
  best_practices_ready: true,
  filtering_enabled: true,
  active_filter_lists: 11,
  total_filter_lists: 11,
  protection_enabled: true,
  protection_disabled_duration_ms: 0,
};

function wrapper({ children }: { children: ReactNode }) {
  return <I18nProvider>{children}</I18nProvider>;
}

// Fake timers freeze testing-library's own waitFor polling, so tests below
// flush pending promises/effects explicitly (advancing by 0ms) instead of
// using waitFor.
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
  vi.spyOn(client, "fetchInstallationStatus").mockResolvedValue(installed);
  vi.spyOn(client, "fetchAdGuardStatus").mockResolvedValue(baseStatus);
  vi.spyOn(client, "bootstrapAdGuard").mockResolvedValue(baseStatus);
  vi.spyOn(client, "fetchAdGuardFilterReport").mockResolvedValue({ passed: 3, expected: 3, blocked: 3, checked_at: new Date().toISOString(), checks: [] });
  vi.spyOn(client, "setAdGuardFiltering").mockResolvedValue(baseStatus);
  vi.spyOn(client, "setAdGuardProtection").mockResolvedValue(baseStatus);
});

afterEach(() => {
  vi.useRealTimers();
});

describe("useAdGuardData", () => {
  it("loads installation and AdGuard status on mount", async () => {
    const { result } = renderHook(() => useAdGuardData(), { wrapper });
    await flush();
    expect(result.current.loading).toBe(false);
    expect(result.current.status?.configured).toBe(true);
    expect(result.current.ready).toBe(true);
  });

  it("does not fetch AdGuard status when not installed", async () => {
    vi.spyOn(client, "fetchInstallationStatus").mockResolvedValue(notInstalled);
    const { result } = renderHook(() => useAdGuardData(), { wrapper });
    await flush();
    expect(result.current.status).toBeNull();
    expect(client.fetchAdGuardStatus).not.toHaveBeenCalled();
  });

  it("reports a load error via errorMessage's fallback", async () => {
    vi.spyOn(client, "fetchInstallationStatus").mockRejectedValue(new Error("boom"));
    const { result } = renderHook(() => useAdGuardData(), { wrapper });
    await flush();
    expect(result.current.error).not.toBe("");
    expect(result.current.loading).toBe(false);
  });

  it("initialize bootstraps AdGuard and reports success", async () => {
    const { result } = renderHook(() => useAdGuardData(), { wrapper });
    await flush();
    await act(async () => {
      await result.current.initialize();
    });
    expect(client.bootstrapAdGuard).toHaveBeenCalledTimes(1);
    expect(result.current.bootstrapping).toBe(false);
    expect(result.current.message).not.toBe("");
  });

  it("toggleFiltering flips filtering_enabled through the API and refreshes status", async () => {
    const { result } = renderHook(() => useAdGuardData(), { wrapper });
    await flush();
    await act(async () => {
      await result.current.toggleFiltering();
    });
    expect(client.setAdGuardFiltering).toHaveBeenCalledWith(false);
    expect(result.current.filteringBusy).toBe(false);
  });

  it("toggleFiltering is a no-op without a loaded status", async () => {
    vi.spyOn(client, "fetchInstallationStatus").mockResolvedValue(notInstalled);
    const { result } = renderHook(() => useAdGuardData(), { wrapper });
    await flush();
    await act(async () => {
      await result.current.toggleFiltering();
    });
    expect(client.setAdGuardFiltering).not.toHaveBeenCalled();
  });

  it("changeProtection resets protectionChoice immediately and calls the API with the right duration", async () => {
    const { result } = renderHook(() => useAdGuardData(), { wrapper });
    await flush();
    await act(async () => {
      await result.current.changeProtection("10m");
    });
    expect(client.setAdGuardProtection).toHaveBeenCalledWith(false, 600);
    expect(result.current.protectionChoice).toBe("");
    expect(result.current.protectionBusy).toBe(false);
  });

  it("testFilters loads a filter report and clears testingFilters afterward", async () => {
    const { result } = renderHook(() => useAdGuardData(), { wrapper });
    await flush();
    await act(async () => {
      await result.current.testFilters();
    });
    expect(client.fetchAdGuardFilterReport).toHaveBeenCalledTimes(1);
    expect(result.current.filterReport?.passed).toBe(3);
    expect(result.current.testingFilters).toBe(false);
  });

  it("polls AdGuard status every 5s only while protection is paused", async () => {
    vi.spyOn(client, "fetchAdGuardStatus").mockResolvedValue({ ...baseStatus, protection_enabled: false, protection_disabled_duration_ms: 600_000 });
    renderHook(() => useAdGuardData(), { wrapper });
    await flush();
    const callsBefore = vi.mocked(client.fetchAdGuardStatus).mock.calls.length;
    await advance(5_000);
    expect(vi.mocked(client.fetchAdGuardStatus).mock.calls.length).toBeGreaterThan(callsBefore);
  });

  it("does not poll AdGuard status while protection is active", async () => {
    renderHook(() => useAdGuardData(), { wrapper });
    await flush();
    const callsBefore = vi.mocked(client.fetchAdGuardStatus).mock.calls.length;
    await advance(20_000);
    expect(vi.mocked(client.fetchAdGuardStatus).mock.calls.length).toBe(callsBefore);
  });
});
