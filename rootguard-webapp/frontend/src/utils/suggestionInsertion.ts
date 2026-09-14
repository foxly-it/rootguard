// Moved out of UnboundExpertEditor.tsx (a component file can only export
// components under the project's react-refresh lint rule) so the pure
// splicing logic can be unit-tested directly.
//
// Splices a catalog example in over the partially-typed directive name
// (prefix) that precedes the cursor, reindenting it to match whatever
// whitespace already led into that name on its line.
//
// Found in a v1.0.0 correctness review: the previous version only
// re-added that leading indentation for a single-line example
// (reference.example.includes("\n") ? ... : indentation + ...) - but the
// indentation was already unconditionally stripped out of `content`
// beforehand (start - indentation.length), regardless of that check. A
// multi-line example (forward-zone:/stub-zone: in Core's own catalog both
// are) landed with the line's original indentation deleted and never
// restored - a real, silent content-deletion bug, not just a cosmetic
// one, on the very first keystroke of using either suggestion while
// indented.
export function computeSuggestionInsertion(content: string, cursor: number, prefix: string, example: string) {
  const start = Math.max(0, cursor - prefix.length);
  const indentation = content.slice(content.lastIndexOf("\n", start - 1) + 1, start).match(/^\s*/)?.[0] ?? "";
  const reindented = indentation + example.trimStart();
  const next = content.slice(0, start - indentation.length) + reindented + content.slice(cursor);
  const nextCursor = start - indentation.length + reindented.length;
  return { next, nextCursor };
}
