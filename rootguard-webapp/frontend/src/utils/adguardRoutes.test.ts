import { describe, expect, it } from "vitest";
import { adguardFiltersHash } from "./adguardRoutes";

describe("adguardFiltersHash", () => {
  it.each(["v0.107.80", "v0.108.0-b.90", "0.107.80"])("keeps the pre-1.0 route on %s", (version) => {
    expect(adguardFiltersHash(version)).toBe("#filters");
  });

  it.each(["v1.0.0", "v1.0.0-b.2", "v1.1.0", "v2.0.0", "1.0.0"])("uses the rewritten UI's route on %s", (version) => {
    expect(adguardFiltersHash(version)).toBe("#/blocklists");
  });

  it.each([undefined, "", "unknown"])("falls back to the current route on %s", (version) => {
    expect(adguardFiltersHash(version)).toBe("#/blocklists");
  });
});
