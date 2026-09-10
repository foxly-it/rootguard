import { describe, expect, it, vi, beforeEach } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { I18nProvider } from "../i18n/provider";
import Overview from "./Overview";
import * as client from "../api/client";

const baseDashboard: client.DashboardResponse = {
  docker: { cpu: 12.5, memory: 1024, metrics_available: true, containers: 5, status: "healthy", collected_at: 1 },
  dns: { status: "healthy", resolver: "unbound", dnssec: true },
};

const installedInstallation: client.InstallationStatus = {
  state: "installed",
  config: { dns_bind_address: "0.0.0.0", dns_port: 53, adguard_channel: "stable", blockpage_enabled: true },
  steps: [],
  updated_at: new Date().toISOString(),
};

const adGuardService: client.ServiceInfo = {
  name: "adguard",
  displayName: "AdGuard Home",
  description: "",
  status: "running",
  health: "healthy",
  restartCount: 0,
  immutable: true,
  metadata: "complete",
  attestation: "verified",
};

const protectedAdGuard: client.AdGuardStatus = {
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

function renderPage() {
  return render(
    <MemoryRouter>
      <I18nProvider>
        <Overview />
      </I18nProvider>
    </MemoryRouter>,
  );
}

beforeEach(() => {
  vi.restoreAllMocks();
  window.localStorage.setItem("rootguard.locale", "de");
  vi.spyOn(client, "fetchDashboard").mockResolvedValue(baseDashboard);
  vi.spyOn(client, "fetchInstallationStatus").mockResolvedValue(installedInstallation);
  vi.spyOn(client, "fetchServices").mockResolvedValue([adGuardService]);
  vi.spyOn(client, "fetchAdGuardStatus").mockResolvedValue(protectedAdGuard);
  vi.spyOn(client, "serviceAction").mockResolvedValue(undefined);
});

// Overview() is now a thin presentation layer over useOverviewData() (found
// in review: data-fetching and JSX used to be mixed into one component) -
// this exercises the wiring between the two, not the data logic itself
// (see useOverviewData.test.tsx for that).
describe("Overview page", () => {
  it("renders the protected headline once every signal is healthy", async () => {
    // Real timers here: useOverviewData's own first tick can't yet know
    // installation is "installed" (loadStatus hasn't resolved when
    // loadAdGuardMetrics's first call checks it - see useOverviewData.ts),
    // so "PROTECTED" only appears after the AdGuard 5s poll actually
    // fires once installationRef has caught up.
    vi.useFakeTimers();
    try {
      renderPage();
      await act(async () => {
        await vi.advanceTimersByTimeAsync(5_000);
      });
      expect(screen.getByText("PROTECTED")).toBeInTheDocument();
      expect(screen.getByText("AdGuard Home")).toBeInTheDocument();
    } finally {
      vi.useRealTimers();
    }
  });

  it("restarts a service via its card button", async () => {
    renderPage();
    await waitFor(() => expect(screen.getByText("AdGuard Home")).toBeInTheDocument());
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: /AdGuard Home/i }));
    await waitFor(() => expect(client.serviceAction).toHaveBeenCalledWith("adguard", "restart"));
  });
});
