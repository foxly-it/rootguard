import { describe, expect, it, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { I18nProvider } from "../i18n/provider";
import Setup from "./Setup";
import * as client from "../api/client";

const baseInstallation: client.InstallationStatus = {
  state: "not_installed",
  steps: [],
  updated_at: new Date().toISOString(),
};

const baseConfig: client.InstallationConfig = {
  dns_bind_address: "192.168.1.2",
  dns_port: 53,
  adguard_channel: "stable",
  blockpage_enabled: true,
};

function renderPage() {
  return render(
    <MemoryRouter>
      <I18nProvider>
        <Setup />
      </I18nProvider>
    </MemoryRouter>,
  );
}

beforeEach(() => {
  vi.restoreAllMocks();
  // jsdom reports navigator.language as en-US regardless of the host
  // machine, which I18nProvider picks by default with no stored
  // preference - pinning it explicitly keeps this deterministic rather
  // than depending on that default (see AdGuard.test.tsx's own comment
  // for the "de" equivalent of this).
  window.localStorage.setItem("rootguard.locale", "en");
  vi.spyOn(client, "fetchInstallationStatus").mockResolvedValue(baseInstallation);
});

// Found in review, round 14: the round-13 preflight advisory (a check
// with ok:true, level:"warning") had no test coverage of its own on the
// frontend - only the backend that produces it did. This covers the
// three things that make it a genuine warning rather than either a pass
// or a failure: it renders distinctly from both, its action text stays
// visible (the pre-existing `!check.ok &&` guard would otherwise have
// hidden it - see Setup.tsx's own check-row rendering), and it never
// disables the install button the way a real failed check does.
describe("Setup preflight warning-level check", () => {
  it("shows the warning distinctly, keeps its action visible, and still allows install", async () => {
    vi.spyOn(client, "preflightInstallation").mockResolvedValue({
      ready: true,
      config: baseConfig,
      checks: [
        { id: "docker", code: "docker_reachable", ok: true, message: "Docker Engine is reachable." },
        {
          id: "docker_engine_patch_level",
          code: "docker_engine_cp_cve",
          ok: true,
          level: "warning",
          message: "Docker Engine predates 29.5.1.",
          detail: "29.4.0",
          action: "Upgrade Docker Engine to 29.5.1 or later.",
        },
      ],
    });
    const user = userEvent.setup();
    renderPage();

    const runButton = await screen.findByRole("button", { name: /run preflight/i });
    await user.click(runButton);

    // The real English translation renders here (this check's code has
    // one, matching production - see diagnosticText's own fallback
    // logic), not the mock's plain-English message/action text verbatim
    // - matched on "29.5.1" and the action's own stable opening word
    // rather than the full sentence, since round 14's own third-CVE fix
    // changes "two" to "three" elsewhere in that same sentence.
    const warningText = await screen.findByText(/This Docker Engine predates 29\.5\.1/i);
    const warningRow = warningText.closest<HTMLElement>(".check-row");
    expect(warningRow).not.toBeNull();
    expect(warningRow).toHaveClass("warning");
    expect(warningRow).not.toHaveClass("ok");
    expect(warningRow).not.toHaveClass("failed");

    // The action text must still be visible for a warning-level check,
    // even though check.ok is true - the render guard that used to hide
    // it for any ok check was the actual bug this covers.
    expect(within(warningRow!).getByText(/Upgrade Docker Engine/i)).toBeInTheDocument();

    const installButton = await screen.findByRole("button", { name: /install dns stack/i });
    expect(installButton).toBeEnabled();
  });
});
