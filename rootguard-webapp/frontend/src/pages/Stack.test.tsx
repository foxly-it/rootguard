import { describe, expect, it, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { I18nProvider } from "../i18n/provider";
import Stack from "./Stack";
import * as client from "../api/client";

const idleUpdates: client.UpdateStatus = {
  state: "idle",
  message: "up to date",
  services: [
    { name: "adguard", display_name: "AdGuard Home", target_image: "adguard/adguardhome:v0.107.79", update_available: false },
  ],
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

function renderPage() {
  return render(
    <MemoryRouter>
      <I18nProvider>
        <Stack />
      </I18nProvider>
    </MemoryRouter>,
  );
}

beforeEach(() => {
  vi.restoreAllMocks();
  window.localStorage.setItem("rootguard.locale", "de");
  vi.spyOn(client, "fetchUpdateStatus").mockResolvedValue(idleUpdates);
  vi.spyOn(client, "fetchControlPlaneUpdateStatus").mockResolvedValue(idleControlPlane);
  vi.spyOn(client, "fetchUpdaterSelfUpdateStatus").mockResolvedValue(idleUpdaterUpdate);
  vi.spyOn(client, "fetchServices").mockResolvedValue([]);
  vi.spyOn(client, "checkUpdates").mockResolvedValue(idleUpdates);
  vi.spyOn(client, "checkControlPlaneUpdates").mockResolvedValue(idleControlPlane);
  vi.spyOn(client, "checkUpdaterSelfUpdate").mockResolvedValue(idleUpdaterUpdate);
});

// Stack() is now a thin presentation layer over useStackData() (found in
// review: data-fetching and JSX used to be mixed into one component) -
// this exercises the wiring between the two, not the data logic itself
// (see useStackData.test.tsx for that).
describe("Stack page", () => {
  it("renders the loaded service card", async () => {
    renderPage();
    await waitFor(() => expect(screen.getByText("AdGuard Home")).toBeInTheDocument());
  });

  it("runs a status check via the header button", async () => {
    renderPage();
    await waitFor(() => expect(screen.getByText("AdGuard Home")).toBeInTheDocument());
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: /Auf Updates prüfen/i }));
    await waitFor(() => expect(client.checkUpdates).toHaveBeenCalledTimes(1));
  });
});
