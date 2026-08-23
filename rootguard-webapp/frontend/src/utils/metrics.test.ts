import { describe, expect, it } from "vitest";
import { blockRatePercent, pushHistory } from "./metrics";

describe("pushHistory", () => {
  it("ignores null samples instead of pushing a placeholder", () => {
    const previous = [{ value: 1, sampledAt: 1000 }];
    expect(pushHistory(previous, null, 24, 2000)).toBe(previous);
  });

  it("records the exact sampledAt it was given, not a derived/uniform interval", () => {
    const result = pushHistory([], 42, 24, 12_345);
    expect(result).toEqual([{ value: 42, sampledAt: 12_345 }]);
  });

  it("keeps irregular real-world gaps between samples intact", () => {
    let history = pushHistory([], 1, 24, 0);
    history = pushHistory(history, 2, 24, 2_000);
    history = pushHistory(history, 3, 24, 4_000);
    history = pushHistory(history, 4, 24, 7_000);
    expect(history.map((point) => point.sampledAt)).toEqual([0, 2_000, 4_000, 7_000]);
  });

  it("trims from the front once maxLength is exceeded", () => {
    let history: ReturnType<typeof pushHistory> = [];
    for (let i = 0; i < 5; i++) history = pushHistory(history, i, 3, i);
    expect(history.map((point) => point.value)).toEqual([2, 3, 4]);
  });
});

describe("blockRatePercent", () => {
  it("returns 0 for zero queries instead of dividing by zero", () => {
    expect(blockRatePercent(0, 0)).toBe(0);
  });

  it("computes the blocked share as a percentage", () => {
    expect(blockRatePercent(25, 100)).toBe(25);
  });
});
