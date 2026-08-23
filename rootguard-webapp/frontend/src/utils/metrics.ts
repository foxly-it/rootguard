export interface HistoryPoint {
  value: number;
  sampledAt: number;
}

// `now` is an explicit parameter (defaulting to Date.now()) rather than
// read internally, purely so tests can pass a fixed value instead of
// mocking global timers/Date for what's otherwise a trivial pure function.
export function pushHistory(previous: HistoryPoint[], value: number | null, maxLength: number, now: number = Date.now()): HistoryPoint[] {
  if (value === null) return previous;
  return [...previous, { value, sampledAt: now }].slice(-maxLength);
}

export function blockRatePercent(blocked: number, queries: number): number {
  return queries === 0 ? 0 : (blocked / queries) * 100;
}
