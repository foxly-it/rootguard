import { describe, expect, it } from "vitest";
import { computeSuggestionInsertion } from "./suggestionInsertion";

describe("computeSuggestionInsertion", () => {
  it("reindents a single-line example to match the line's existing indentation", () => {
    const draft = "server:\n    veri";
    const cursor = draft.length;
    const { next, nextCursor } = computeSuggestionInsertion(draft, cursor, "veri", "verbosity: 1");
    expect(next).toBe("server:\n    verbosity: 1");
    expect(nextCursor).toBe(next.length);
  });

  // Regression test for a v1.0.0 correctness review finding: the
  // indentation preceding the typed prefix used to be stripped out
  // unconditionally, but only restored for a single-line example -
  // silently deleting real content (the line's indentation) whenever the
  // selected suggestion's own example spanned multiple lines, as both
  // forward-zone: and stub-zone: do in Core's own catalog.
  it("does not delete the line's indentation when the example spans multiple lines", () => {
    const draft = "server:\n    forw";
    const cursor = draft.length;
    const { next } = computeSuggestionInsertion(draft, cursor, "forw", "forward-zone:\n    name: \"corp.example.\"");
    expect(next).toBe('server:\n    forward-zone:\n    name: "corp.example."');
  });

  it("leaves a top-level (unindented) multi-line example untouched", () => {
    const draft = "forw";
    const cursor = draft.length;
    const { next } = computeSuggestionInsertion(draft, cursor, "forw", "forward-zone:\n    name: \"corp.example.\"");
    expect(next).toBe('forward-zone:\n    name: "corp.example."');
  });
});
