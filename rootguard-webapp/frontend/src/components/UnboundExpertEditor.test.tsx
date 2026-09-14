import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { I18nProvider } from "../i18n/provider";
import { useI18n } from "../i18n";
import * as client from "../api/client";
import UnboundExpertEditor from "./UnboundExpertEditor";

// Renders the editor alongside a locale switcher, mirroring how the real
// app's language control lives elsewhere in the component tree but still
// shares the same I18nProvider - all this test needs to reproduce the
// bug is a way to flip locale without touching UnboundExpertEditor itself.
function LocaleSwitcher() {
  const { setLocale } = useI18n();
  return <button type="button" onClick={() => setLocale("de")}>switch to German</button>;
}

function Harness({ onActivated }: { onActivated: () => Promise<void> }) {
  return (
    <I18nProvider>
      <LocaleSwitcher />
      <UnboundExpertEditor onActivated={onActivated} />
    </I18nProvider>
  );
}

describe("UnboundExpertEditor", () => {
  // Regression test for a v1.0.0 correctness review finding: the
  // data-loading effect depended on t directly, whose identity changes on
  // every locale switch (I18nProvider's own t depends on locale) -
  // switching the app's language reran load() and silently discarded
  // whatever the operator had typed into the draft textarea, reloading
  // the last-saved server content over it.
  it("does not discard an in-progress draft when the UI language changes", async () => {
    vi.spyOn(client, "fetchUnboundCustom").mockResolvedValue({ content: "server:\n    hide-identity: yes\n", max_bytes: 65536 });
    vi.spyOn(client, "fetchUnboundDirectives").mockResolvedValue([]);

    const user = userEvent.setup();
    render(<Harness onActivated={() => Promise.resolve()} />);

    await act(async () => {});
    await user.click(screen.getByRole("button", { name: "Open editor" }));

    const textarea = await screen.findByLabelText("Additional Unbound configuration");
    await user.type(textarea, "\n    hide-version: yes");
    const draftBeforeSwitch = (textarea as HTMLTextAreaElement).value;

    await user.click(screen.getByRole("button", { name: "switch to German" }));
    await act(async () => {});

    expect((textarea as HTMLTextAreaElement).value).toBe(draftBeforeSwitch);
  });
});
