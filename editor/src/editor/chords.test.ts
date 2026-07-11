import { describe, expect, it } from "vitest";
import { filterChords, CHORD_LIST } from "./chords";

describe("filterChords", () => {
  it("returns the full list for an empty query", () => {
    expect(filterChords("")).toEqual(CHORD_LIST);
  });

  it("filters case-insensitively by substring", () => {
    const result = filterChords("bm7");
    expect(result).toContain("Bm7");
    expect(result.every((c) => c.toLowerCase().includes("bm7"))).toBe(true);
  });
});
