import { render, screen, act } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router";
import { describe, expect, it, vi, beforeEach } from "vitest";
import type { ReactNode } from "react";
import { I18nProvider } from "../i18n/provider";
import Backups from "./Backups";
import * as client from "../api/client";

const baseBackups: client.BackupStatus = {
  settings: { retention_per_service: 3 },
  count: 0,
  managed_bytes: 0,
  unmanaged_bytes: 0,
  services: [],
};

const baseUpdates: client.UpdateStatus = {
  state: "idle",
  message: "up to date",
  services: [],
  updated_at: new Date().toISOString(),
};

const basePreview: client.BackupRestorePreview = {
  schema_version: 1,
  created_at: new Date().toISOString(),
  file_count: 5,
  expanded_bytes: 1024,
  config: { dns_bind_address: "192.0.2.10", dns_port: 53, adguard_channel: "stable", blockpage_enabled: false },
  preflight: {
    ready: true,
    config: { dns_bind_address: "192.0.2.10", dns_port: 53, adguard_channel: "stable", blockpage_enabled: false },
    checks: [],
  },
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
  vi.spyOn(client, "fetchBackupStatus").mockResolvedValue(baseBackups);
  vi.spyOn(client, "fetchUpdateStatus").mockResolvedValue(baseUpdates);
  vi.spyOn(client, "previewEncryptedBackup").mockResolvedValue(basePreview);
});

async function renderWithPreview() {
  const user = userEvent.setup();
  render(<Backups />, { wrapper });

  const restoreSection = document.getElementById("backup-restore") as HTMLElement;
  const file = new File(["encrypted"], "backup.age", { type: "application/vnd.rootguard.backup+age" });
  const fileInput = restoreSection.querySelector('input[type="file"]') as HTMLInputElement;
  await act(async () => {
    await user.upload(fileInput, file);
  });
  const passphraseInput = restoreSection.querySelector('input[type="password"]') as HTMLInputElement;
  await act(async () => {
    await user.type(passphraseInput, "a-very-long-passphrase");
  });
  await act(async () => {
    await user.click(screen.getByRole("button", { name: "Validate backup" }));
  });

  return user;
}

describe("Backups restore preview port field", () => {
  // Regression test for a v1.0.0 correctness review finding:
  // event.target.valueAsNumber is NaN while a number input is empty (the
  // operator selects-all and deletes to type a new port), and this used
  // to be stored straight into config.dns_port unconditionally - which
  // also force-flipped preflight.ready to false (as every real edit
  // should), showing a "Recheck target" button for a config that hadn't
  // actually changed to anything valid. With the fix, a NaN edit is
  // ignored entirely, so the field can look momentarily blank while
  // typing without ever corrupting the actual submitted config or
  // invalidating an otherwise-still-valid preview.
  it("does not invalidate a valid preview when the port field is cleared mid-edit", async () => {
    const user = await renderWithPreview();

    const portInput = await screen.findByDisplayValue("53") as HTMLInputElement;
    await act(async () => {
      await user.clear(portInput);
    });

    expect(screen.queryByRole("button", { name: "Recheck target" })).not.toBeInTheDocument();
  });

  it("still invalidates the preview for a genuine port change", async () => {
    const user = await renderWithPreview();

    const portInput = await screen.findByDisplayValue("53") as HTMLInputElement;
    await act(async () => {
      await user.clear(portInput);
      await user.type(portInput, "80");
    });

    expect(screen.getByRole("button", { name: "Recheck target" })).toBeInTheDocument();
  });
});
