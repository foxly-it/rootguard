import { describe, expect, it, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { I18nProvider } from "../i18n/provider";
import Unbound from "./Unbound";
import * as client from "../api/client";

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
  forward_zones: [],
  private_domains: [],
  reverse_zones: [],
  local_zones: [],
};

function renderPage() {
  return render(
    <MemoryRouter>
      <I18nProvider>
        <Unbound />
      </I18nProvider>
    </MemoryRouter>,
  );
}

// jsdom doesn't implement IntersectionObserver - UnboundSectionNav (the
// scroll-spy sub-nav registered via useSidebarSubNav) uses one to track
// which section is currently in view. Not this page's own logic to
// verify (see the useUnboundData tests for that), just a browser API the
// render needs present to not crash.
class MockIntersectionObserver implements IntersectionObserver {
  readonly root = null;
  readonly rootMargin = "";
  readonly thresholds: number[] = [];
  observe() {}
  unobserve() {}
  disconnect() {}
  takeRecords(): IntersectionObserverEntry[] { return []; }
}

beforeEach(() => {
  vi.restoreAllMocks();
  vi.stubGlobal("IntersectionObserver", MockIntersectionObserver);
  window.localStorage.setItem("rootguard.locale", "de");
  vi.spyOn(client, "fetchUnboundSettings").mockResolvedValue(baseSettings);
  vi.spyOn(client, "fetchUnboundHistory").mockResolvedValue([]);
  vi.spyOn(client, "fetchUnboundPresets").mockResolvedValue([]);
  vi.spyOn(client, "fetchUnboundActiveConfiguration").mockResolvedValue({
    base_config: "", managed_config: "", custom_config: "", checked_at: new Date().toISOString(),
  });
  vi.spyOn(client, "fetchUnboundDiagnosticLoggingStatus").mockResolvedValue({ active: false, level: 0 });
  vi.spyOn(client, "fetchUnboundAdvice").mockResolvedValue({ status: "optimized", recommendations: [] });
});

// Unbound() is now a thin presentation layer over useUnboundData() (found
// in review: data-fetching and JSX were mixed into one component) - this
// exercises the wiring between the two, not the data logic itself (see
// useUnboundData.test.tsx for that). Routing (active tab) and the
// read-only config modal stay page-local state, not the hook's concern.
describe("Unbound page", () => {
  it("loads settings and renders the overview tab by default", async () => {
    renderPage();
    await waitFor(() => expect(screen.queryByText(/werden geladen/i)).not.toBeInTheDocument());
    expect(screen.getByRole("tab", { name: /Übersicht/i })).toHaveAttribute("aria-selected", "true");
  });
});
