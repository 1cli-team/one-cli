import { describe, expect, it } from "vitest";
import { clamp, inRange } from "./index";

describe("clamp", () => {
  it("limits value into given boundaries", () => {
    expect(clamp(10, 0, 5)).toBe(5);
    expect(clamp(-3, 0, 5)).toBe(0);
    expect(clamp(3, 0, 5)).toBe(3);
  });

  it("throws when min is greater than max", () => {
    expect(() => clamp(1, 5, 0)).toThrowError("min must be less than or equal to max");
  });
});

describe("inRange", () => {
  it("returns whether value is inside a closed range", () => {
    expect(inRange(3, { start: 1, end: 5 })).toBe(true);
    expect(inRange(6, { start: 1, end: 5 })).toBe(false);
  });
});
