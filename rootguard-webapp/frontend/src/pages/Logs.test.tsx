import { render, screen, act, fireEvent } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { describe, expect, it, vi, beforeEach } from "vitest";
import type { ReactNode } from "react";
import { I18nProvider } from "../i18n/provider";
import Logs from "./Logs";
import * as client from "../api/client";

const services: client.ServiceInfo[] = [
  { name: "core", displayName: "Core", description: "", status: "running", health: "healthy", restartCount: 0, immutable: true, metadata: "complete", attestation: "verified" },
];

const logs: client.ServiceLogs = {
  service: "core",
  lines: ["a log line"],
  tail: 200,
  since: "1h",
  truncated: false,
  redacted: false,
  description: "",
};

function wrapper({ children }: { children: ReactNode }) {
  return (
    <MemoryRouter>
      <I18nProvider>{children}</I18nProvider>
    </MemoryRouter>
  );
}

beforeEach(() => {
  vi.restoreAllMocks();
  vi.spyOn(client, "fetchServices").mockResolvedValue(services);
  vi.spyOn(client, "fetchServiceLogs").mockResolvedValue(logs);
  vi.stubGlobal("URL", Object.assign(URL, {
    createObjectURL: vi.fn(() => "blob:mock-url"),
    revokeObjectURL: vi.fn(),
  }));
  vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => {});
});

// Regression test for a v1.0.0 correctness review finding:
// URL.revokeObjectURL(href) used to run in the very same tick as
// anchor.click(), racing the browser's own (asynchronous) read of the
// blob: URL to actually perform the download - most notably broken in
// Safari, intermittently producing an empty or failed download.
describe("Logs download report", () => {
  it("defers revoking the object URL to a later tick instead of revoking it immediately", async () => {
    render(<Logs />, { wrapper });

    const downloadButton = await screen.findByRole("button", { name: /Diagnostic report/i });

    // fireEvent dispatches synchronously - unlike userEvent.click, whose
    // own internal implementation awaits enough microtasks that a
    // setTimeout(..., 0) scheduled inside the handler can already have
    // fired before its promise resolves, which would make this ordering
    // assertion meaningless.
    act(() => {
      fireEvent.click(downloadButton);
    });

    expect(URL.createObjectURL).toHaveBeenCalledTimes(1);
    expect(HTMLAnchorElement.prototype.click).toHaveBeenCalledTimes(1);
    // Nothing between the click above and here yields to the event loop,
    // so a setTimeout(..., 0) scheduled inside the click handler cannot
    // have fired yet - this is exactly what distinguishes a deferred
    // revoke from an immediate one.
    expect(URL.revokeObjectURL).not.toHaveBeenCalled();

    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 0));
    });

    expect(URL.revokeObjectURL).toHaveBeenCalledWith("blob:mock-url");
  });
});
