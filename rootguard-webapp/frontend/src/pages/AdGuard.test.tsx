import { describe, expect, it, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { I18nProvider } from "../i18n/provider";
import AdGuard from "./AdGuard";
import * as client from "../api/client";

const baseStatus: client.AdGuardStatus = {
  configured: true,
  healthy: true,
  version: "v0.107.79",
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

const baseInstallation: client.InstallationStatus = {
  state: "installed",
  steps: [],
  updated_at: new Date().toISOString(),
};

function renderPage() {
  return render(
    <MemoryRouter>
      <I18nProvider>
        <AdGuard />
      </I18nProvider>
    </MemoryRouter>,
  );
}

beforeEach(() => {
  vi.restoreAllMocks();
  // I18nProvider otherwise picks navigator.language, which jsdom reports as
  // en-US regardless of the host machine - pinning it keeps the German
  // label text these tests match against ("DNS-Filterung" etc.)
  // deterministic across environments instead of depending on that default.
  window.localStorage.setItem("rootguard.locale", "de");
  vi.spyOn(client, "fetchInstallationStatus").mockResolvedValue(baseInstallation);
  vi.spyOn(client, "fetchAdGuardStatus").mockResolvedValue(baseStatus);
});

describe("AdGuard filtering switch", () => {
  it("reflects filtering_enabled as the on/off visual state", async () => {
    renderPage();
    const checkbox = await screen.findByRole("checkbox", { name: /DNS-Filterung/i });
    await waitFor(() => expect(checkbox).toBeChecked());
    expect(checkbox.closest(".adguard-switch")).toHaveClass("is-on");
  });

  it("toggles via a mouse click and calls setAdGuardFiltering with the flipped value", async () => {
    const setFiltering = vi
      .spyOn(client, "setAdGuardFiltering")
      .mockResolvedValue({ ...baseStatus, filtering_enabled: false });
    const user = userEvent.setup();
    renderPage();
    const checkbox = await screen.findByRole("checkbox", { name: /DNS-Filterung/i });
    await waitFor(() => expect(checkbox).toBeChecked());

    await user.click(checkbox);

    expect(setFiltering).toHaveBeenCalledWith(false);
    await waitFor(() => expect(checkbox).not.toBeChecked());
    expect(checkbox.closest(".adguard-switch")).toHaveClass("is-off");
  });

  it("toggles via the keyboard (Space) the same as a click, since it's a real checkbox underneath", async () => {
    const setFiltering = vi
      .spyOn(client, "setAdGuardFiltering")
      .mockResolvedValue({ ...baseStatus, filtering_enabled: false });
    const user = userEvent.setup();
    renderPage();
    const checkbox = await screen.findByRole("checkbox", { name: /DNS-Filterung/i });
    await waitFor(() => expect(checkbox).toBeChecked());

    checkbox.focus();
    await user.keyboard(" ");

    expect(setFiltering).toHaveBeenCalledWith(false);
  });

  it("disables the checkbox while a toggle request is in flight, and re-enables it once it settles", async () => {
    let resolveToggle: (value: client.AdGuardStatus) => void = () => {};
    const pending = new Promise<client.AdGuardStatus>((resolve) => {
      resolveToggle = resolve;
    });
    vi.spyOn(client, "setAdGuardFiltering").mockReturnValue(pending);
    const user = userEvent.setup();
    renderPage();
    const checkbox = await screen.findByRole("checkbox", { name: /DNS-Filterung/i });
    await waitFor(() => expect(checkbox).toBeChecked());

    await user.click(checkbox);
    expect(checkbox).toBeDisabled();

    resolveToggle({ ...baseStatus, filtering_enabled: false });
    await waitFor(() => expect(checkbox).not.toBeDisabled());
  });
});

describe("AdGuard protection select", () => {
  it("triggers the chosen action with the right duration and resets back to the placeholder", async () => {
    const setProtection = vi
      .spyOn(client, "setAdGuardProtection")
      .mockResolvedValue({ ...baseStatus, protection_enabled: false, protection_disabled_duration_ms: 600_000 });
    const user = userEvent.setup();
    renderPage();
    const select = await screen.findByRole("combobox", { name: /AdGuard-Schutz/i });

    await user.selectOptions(select, "10m");

    expect(setProtection).toHaveBeenCalledWith(false, 600);
    // The select is an action trigger, not a state display (see the comment
    // above changeProtection in AdGuard.tsx) - it must snap back to the
    // placeholder immediately rather than staying on "10m", or a second,
    // shorter pause chosen right after would look like a no-op change.
    await waitFor(() => expect(select).toHaveValue(""));
  });

  it("disables the select while a protection change is in flight", async () => {
    let resolveChange: (value: client.AdGuardStatus) => void = () => {};
    const pending = new Promise<client.AdGuardStatus>((resolve) => {
      resolveChange = resolve;
    });
    vi.spyOn(client, "setAdGuardProtection").mockReturnValue(pending);
    const user = userEvent.setup();
    renderPage();
    const select = await screen.findByRole("combobox", { name: /AdGuard-Schutz/i });

    await user.selectOptions(select, "off");
    expect(select).toBeDisabled();

    resolveChange({ ...baseStatus, protection_enabled: false, protection_disabled_duration_ms: 0 });
    await waitFor(() => expect(select).not.toBeDisabled());
  });
});
