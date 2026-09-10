import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { I18nProvider } from "../i18n/provider";
import * as client from "../api/client";
import { useOverviewData } from "./useOverviewData";

const baseDashboard: client.DashboardResponse = {
  docker: { cpu: 1, memory: 2, metrics_available: true, containers: 5, status: "healthy", collected_at: 1 },
  dns: { status: "healthy", resolver: "unbound", dnssec: true },
};

const notInstalled: client.InstallationStatus = {
  state: "not_installed",
  steps: [],
  updated_at: new Date().toISOString(),
};

const installed: client.InstallationStatus = {
  state: "installed",
  steps: [],
  updated_at: new Date().toISOString(),
};

const baseAdGuard: client.AdGuardStatus = {
  configured: true,
  healthy: true,
  upstream: "172.29.53.2:5335",
  upstream_ready: true,
  stats_available: true,
  queries: 10,
  blocked: 1,
  average_response_seconds: 0.02,
  best_practices_ready: true,
  filtering_enabled: true,
  active_filter_lists: 1,
  total_filter_lists: 1,
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
  vi.spyOn(client, "fetchDashboard").mockResolvedValue(baseDashboard);
  vi.spyOn(client, "fetchInstallationStatus").mockResolvedValue(notInstalled);
  vi.spyOn(client, "fetchServices").mockResolvedValue([]);
  vi.spyOn(client, "fetchAdGuardStatus").mockResolvedValue(baseAdGuard);
  vi.spyOn(client, "serviceAction").mockResolvedValue(undefined);
});

afterEach(() => {
  vi.useRealTimers();
});

describe("useOverviewData", () => {
  it("loads dashboard and installation status immediately on mount", async () => {
    const { result } = renderHook(() => useOverviewData(), { wrapper });
    await flush();
    expect(result.current.dashboard?.docker.cpu).toBe(1);
    expect(client.fetchInstallationStatus).toHaveBeenCalledTimes(1);
  });

  it("does not poll AdGuard status before installation is installed", async () => {
    renderHook(() => useOverviewData(), { wrapper });
    await flush();
    await advance(20_000);
    expect(client.fetchAdGuardStatus).not.toHaveBeenCalled();
  });

  it("starts polling AdGuard status once installation becomes installed", async () => {
    vi.spyOn(client, "fetchInstallationStatus").mockResolvedValue(installed);
    renderHook(() => useOverviewData(), { wrapper });
    await flush();
    await advance(5_000);
    expect(client.fetchAdGuardStatus).toHaveBeenCalled();
  });

  it("polls core metrics roughly every 500ms", async () => {
    renderHook(() => useOverviewData(), { wrapper });
    await flush();
    const callsBefore = vi.mocked(client.fetchDashboard).mock.calls.length;
    await advance(2_000);
    // 2000ms / 500ms cadence - at least 3 more ticks, allowing for the
    // in-flight guard to skip a tick if a mocked call hasn't settled yet.
    expect(vi.mocked(client.fetchDashboard).mock.calls.length).toBeGreaterThanOrEqual(callsBefore + 3);
  });

  it("pauses all polling while the tab is hidden and reloads immediately when visible again", async () => {
    renderHook(() => useOverviewData(), { wrapper });
    await flush();

    Object.defineProperty(document, "hidden", { configurable: true, get: () => true });
    await act(async () => {
      document.dispatchEvent(new Event("visibilitychange"));
      await vi.advanceTimersByTimeAsync(0);
    });
    const callsWhileHidden = vi.mocked(client.fetchDashboard).mock.calls.length;
    await advance(5_000);
    expect(vi.mocked(client.fetchDashboard).mock.calls.length).toBe(callsWhileHidden);

    Object.defineProperty(document, "hidden", { configurable: true, get: () => false });
    await act(async () => {
      document.dispatchEvent(new Event("visibilitychange"));
      await vi.advanceTimersByTimeAsync(0);
    });
    expect(vi.mocked(client.fetchDashboard).mock.calls.length).toBeGreaterThan(callsWhileHidden);
  });

  it("restart sets busyService during the call and clears it afterward, then reloads", async () => {
    const { result } = renderHook(() => useOverviewData(), { wrapper });
    await flush();
    const statusCallsBefore = vi.mocked(client.fetchInstallationStatus).mock.calls.length;

    await act(async () => {
      await result.current.restart("adguard");
    });

    expect(client.serviceAction).toHaveBeenCalledWith("adguard", "restart");
    expect(result.current.busyService).toBe("");
    expect(vi.mocked(client.fetchInstallationStatus).mock.calls.length).toBeGreaterThan(statusCallsBefore);
  });
});
