import { useEffect, useRef } from "react";

/**
 * Runs `callback` every `delayMs` milliseconds, and not at all while
 * `delayMs` is null - the standard escape hatch for polling that should
 * pause under some condition (see callers, e.g. only polling while a page
 * is visible or an operation is in flight). `callback` is read through a
 * ref that's kept current on every render, so callers don't need to
 * memoize it themselves; only a `delayMs` change (or unmount) tears down
 * and re-creates the interval itself.
 *
 * Found in review: 8 hand-rolled setInterval/clearInterval pairs across 6
 * pages duplicated this exact lifecycle.
 */
export function useInterval(callback: () => void, delayMs: number | null): void {
  const callbackRef = useRef(callback);
  useEffect(() => {
    callbackRef.current = callback;
  }, [callback]);

  useEffect(() => {
    if (delayMs === null) return;
    const id = window.setInterval(() => callbackRef.current(), delayMs);
    return () => window.clearInterval(id);
  }, [delayMs]);
}
