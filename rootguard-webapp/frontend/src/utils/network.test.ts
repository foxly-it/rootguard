import { describe, expect, it } from "vitest";
import { detectDefaultBindAddress } from "./network";

describe("detectDefaultBindAddress", () => {
  it.each([
    ["192.168.1.10", "192.168.1.10"],
    ["10.0.0.5", "10.0.0.5"],
    ["127.0.0.1", "0.0.0.0"],
    ["rootguard.local", "0.0.0.0"],
    ["localhost", "0.0.0.0"],
    ["::1", "0.0.0.0"],
    ["999.1.1.1", "0.0.0.0"],
    ["1.2.3", "0.0.0.0"],
  ])("%s -> %s", (hostname, expected) => {
    expect(detectDefaultBindAddress(hostname)).toBe(expected);
  });
});
