import { describe, expect, it } from "vitest";
import { formatCountdown } from "./countdown";

describe("formatCountdown", () => {
  it("formats whole minutes and seconds as M:SS", () => {
    expect(formatCountdown(9 * 60_000 + 45_000)).toBe("9:45");
  });

  it("pads single-digit seconds with a leading zero", () => {
    expect(formatCountdown(65_000)).toBe("1:05");
  });

  it("rounds up to the next full second instead of truncating", () => {
    expect(formatCountdown(500)).toBe("0:01");
  });

  it("clamps negative remaining time to 0:00 instead of going negative", () => {
    expect(formatCountdown(-5000)).toBe("0:00");
  });

  it("handles durations over an hour", () => {
    expect(formatCountdown(60 * 60_000)).toBe("60:00");
  });
});
