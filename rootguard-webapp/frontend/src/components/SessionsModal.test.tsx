import { act, render, screen } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import { I18nProvider } from "../i18n/provider";
import * as client from "../api/client";
import SessionsModal from "./SessionsModal";

beforeEach(() => {
  vi.restoreAllMocks();
  vi.spyOn(client, "fetchSessions").mockResolvedValue([]);
});

// Regression test for a v1.0.0 correctness review finding: the audit log
// only ever declared/translated the 10 auth-specific events, even though
// /api/auth/audit also returns every one of the 24 destructive-action
// base events (72 more combinations with their own _success/_failure/
// _rate_limited suffixes) - any of those rendered as the raw, untranslated
// "sessions.activity.<event>" key text, and the "warning" highlight
// (previously a hardcoded 5-name Set) never applied to any of them either.
describe("SessionsModal audit log", () => {
  it("translates a destructive-action audit event instead of showing the raw key", async () => {
    vi.spyOn(client, "fetchAuditLog").mockResolvedValue([
      { timestamp: new Date().toISOString(), event: "backup_export_success", remote_ip: "192.0.2.1" },
    ]);

    await act(async () => {
      render(<SessionsModal open onClose={() => {}} />, {
        wrapper: ({ children }) => <I18nProvider>{children}</I18nProvider>,
      });
    });

    expect(screen.getByText("Backup exported")).toBeInTheDocument();
    expect(screen.queryByText(/sessions\.activity\./)).not.toBeInTheDocument();
  });

  it("highlights a destructive-action failure the same way an auth failure is highlighted", async () => {
    vi.spyOn(client, "fetchAuditLog").mockResolvedValue([
      { timestamp: new Date().toISOString(), event: "backup_export_failure", remote_ip: "192.0.2.1" },
    ]);

    await act(async () => {
      render(<SessionsModal open onClose={() => {}} />, {
        wrapper: ({ children }) => <I18nProvider>{children}</I18nProvider>,
      });
    });

    const entry = screen.getByText("Failed backup export attempt").closest("li");
    expect(entry).toHaveClass("audit-entry", "warning");
  });

  it("does not highlight a destructive-action success", async () => {
    vi.spyOn(client, "fetchAuditLog").mockResolvedValue([
      { timestamp: new Date().toISOString(), event: "backup_export_success", remote_ip: "192.0.2.1" },
    ]);

    await act(async () => {
      render(<SessionsModal open onClose={() => {}} />, {
        wrapper: ({ children }) => <I18nProvider>{children}</I18nProvider>,
      });
    });

    const entry = screen.getByText("Backup exported").closest("li");
    expect(entry).toHaveClass("audit-entry");
    expect(entry).not.toHaveClass("warning");
  });
});
