import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { I18nProvider } from "../i18n/provider";
import * as client from "../api/client";
import { useStackData } from "./useStackData";

const idleUpdates: client.UpdateStatus = {
  state: "idle",
  message: "up to date",
  services: [],
  updated_at: new Date().toISOString(),
};

const idleControlPlane: client.ControlPlaneUpdateStatus = {
  state: "idle",
  message: "up to date",
  services: [],
  updated_at: new Date().toISOString(),
};

const idleUpdaterUpdate: client.UpdaterSelfUpdateStatus = {
  state: "idle",
  message: "up to date",
  services: [],
  updated_at: new Date().toISOString(),
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
  vi.spyOn(client, "fetchUpdateStatus").mockResolvedValue(idleUpdates);
  vi.spyOn(client, "fetchControlPlaneUpdateStatus").mockResolvedValue(idleControlPlane);
  vi.spyOn(client, "fetchUpdaterSelfUpdateStatus").mockResolvedValue(idleUpdaterUpdate);
  vi.spyOn(client, "fetchServices").mockResolvedValue([]);
  vi.spyOn(client, "checkUpdates").mockResolvedValue(idleUpdates);
  vi.spyOn(client, "checkControlPlaneUpdates").mockResolvedValue(idleControlPlane);
  vi.spyOn(client, "checkUpdaterSelfUpdate").mockResolvedValue(idleUpdaterUpdate);
  vi.spyOn(client, "installControlPlaneUpdates").mockResolvedValue({ ...idleControlPlane, state: "updating" });
  vi.spyOn(client, "fetchCleanupPreview").mockResolvedValue({ resources: [], estimated_bytes: 0 });
  vi.spyOn(client, "runManualCleanup").mockResolvedValue({});
  vi.spyOn(client, "serviceAction").mockResolvedValue(undefined);
});

afterEach(() => {
  vi.useRealTimers();
});

describe("useStackData", () => {
  it("loads update/control-plane/updater status and services on mount", async () => {
    const { result } = renderHook(() => useStackData(), { wrapper });
    await flush();
    expect(result.current.updates).toEqual(idleUpdates);
    expect(result.current.controlPlane).toEqual(idleControlPlane);
    expect(result.current.updaterUpdate).toEqual(idleUpdaterUpdate);
  });

  it("polls every 10s while idle, and every 1.5s while busy", async () => {
    const { result } = renderHook(() => useStackData(), { wrapper });
    await flush();
    const callsAtIdle = vi.mocked(client.fetchUpdateStatus).mock.calls.length;
    await advance(1_500);
    expect(vi.mocked(client.fetchUpdateStatus).mock.calls.length).toBe(callsAtIdle);
    await advance(8_500);
    expect(vi.mocked(client.fetchUpdateStatus).mock.calls.length).toBeGreaterThan(callsAtIdle);

    // Simulate an update actually starting - fetchUpdateStatus (what the
    // recurring load() itself calls) now reports "updating" too, not just
    // the one-off checkUpdates() call, so busy stays true across the
    // subsequent polls this assertion depends on.
    vi.spyOn(client, "fetchUpdateStatus").mockResolvedValue({ ...idleUpdates, state: "updating" });
    vi.spyOn(client, "checkUpdates").mockResolvedValue({ ...idleUpdates, state: "updating" });
    await act(async () => {
      await result.current.startCheck();
    });
    const callsAtBusy = vi.mocked(client.fetchUpdateStatus).mock.calls.length;
    await advance(1_500);
    expect(vi.mocked(client.fetchUpdateStatus).mock.calls.length).toBeGreaterThan(callsAtBusy);
  });

  it("startControlPlaneUpdate does nothing when the confirm dialog is declined", async () => {
    vi.spyOn(window, "confirm").mockReturnValue(false);
    const { result } = renderHook(() => useStackData(), { wrapper });
    await flush();

    await act(async () => {
      await result.current.startControlPlaneUpdate();
    });

    expect(client.installControlPlaneUpdates).not.toHaveBeenCalled();
  });

  it("startControlPlaneUpdate installs once confirmed", async () => {
    vi.spyOn(window, "confirm").mockReturnValue(true);
    const { result } = renderHook(() => useStackData(), { wrapper });
    await flush();

    await act(async () => {
      await result.current.startControlPlaneUpdate();
    });

    expect(client.installControlPlaneUpdates).toHaveBeenCalledTimes(1);
    expect(result.current.controlPlane?.state).toBe("updating");
  });

  it("control() requires confirmation to stop a service, but not to start or restart one", async () => {
    vi.spyOn(window, "confirm").mockReturnValue(false);
    const { result } = renderHook(() => useStackData(), { wrapper });
    await flush();

    await act(async () => {
      await result.current.control("adguard", "stop");
    });
    expect(client.serviceAction).not.toHaveBeenCalled();

    await act(async () => {
      await result.current.control("adguard", "restart");
    });
    expect(client.serviceAction).toHaveBeenCalledWith("adguard", "restart");
  });

  it("startManualCleanup runs the cleanup and reloads status/preview when confirmed", async () => {
    vi.spyOn(client, "fetchCleanupPreview").mockResolvedValue({
      resources: [{ kind: "image", id: "sha256:abc", estimated_bytes: 1024 }],
      estimated_bytes: 1024,
    });
    vi.spyOn(window, "confirm").mockReturnValue(true);
    const { result } = renderHook(() => useStackData(), { wrapper });
    await flush();

    await act(async () => {
      await result.current.refreshCleanupPreview();
    });
    const reloadsBefore = vi.mocked(client.fetchUpdateStatus).mock.calls.length;

    await act(async () => {
      await result.current.startManualCleanup();
    });

    expect(client.runManualCleanup).toHaveBeenCalledTimes(1);
    expect(result.current.runningCleanup).toBe(false);
    expect(vi.mocked(client.fetchUpdateStatus).mock.calls.length).toBeGreaterThan(reloadsBefore);
  });
});
