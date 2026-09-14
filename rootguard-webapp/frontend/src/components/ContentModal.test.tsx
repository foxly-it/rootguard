import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import ContentModal from "./ContentModal";

// Mirrors the shape of most real callers: onClose is a fresh inline
// closure created on every render of the parent, not a memoized one.
function Harness({ rerenderTrigger }: { rerenderTrigger: number }) {
  const [open] = useState(true);
  return (
    <ContentModal open={open} title="Test modal" closeLabel="Close" onClose={() => {}}>
      <textarea aria-label="modal-textarea" defaultValue="" />
      {/* Forces a genuinely new onClose closure identity without touching
          open, by depending on a prop that changes independently. */}
      <span data-rerender={rerenderTrigger} />
    </ContentModal>
  );
}

describe("ContentModal", () => {
  it("does not steal focus back to the close button when a parent re-render only changes onClose's identity", async () => {
    const user = userEvent.setup();
    const { rerender } = render(<Harness rerenderTrigger={0} />);

    const textarea = screen.getByLabelText("modal-textarea");
    await user.click(textarea);
    await user.type(textarea, "in progress");
    expect(document.activeElement).toBe(textarea);

    // Re-render the parent with a new onClose closure identity, open
    // unchanged - simulates an unrelated state update elsewhere in a real
    // caller while the modal stays open.
    rerender(<Harness rerenderTrigger={1} />);

    expect(document.activeElement).toBe(textarea);
  });
});
