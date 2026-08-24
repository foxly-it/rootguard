export interface HistoryPoint {
  value: number;
  sampledAt: number;
}

// `now` is an explicit parameter (defaulting to Date.now()) rather than
// read internally, purely so tests can pass a fixed value instead of
// mocking global timers/Date for what's otherwise a trivial pure function.
//
// Skipping a push when `now` exactly matches the last recorded sample's
// timestamp handles polling a backend cache faster than it refreshes: the
// dashboard cache backing CPU/RAM updates roughly once a second while the
// frontend polls twice that fast, so passing the cache's own collected_at
// as `now` (instead of the receipt time) means an unchanged cache read
// this way, not a new one under a fresher-looking age.
export function pushHistory(previous: HistoryPoint[], value: number | null, maxLength: number, now: number = Date.now()): HistoryPoint[] {
  if (value === null) return previous;
  if (previous.at(-1)?.sampledAt === now) return previous;
  return [...previous, { value, sampledAt: now }].slice(-maxLength);
}

export function blockRatePercent(blocked: number, queries: number): number {
  return queries === 0 ? 0 : (blocked / queries) * 100;
}
