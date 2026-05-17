import { safeParseJSON } from "./index";

describe("safeParseJSON", () => {
  it("should parse valid JSON string", () => {
    const result = safeParseJSON('{"name":"Alice","age":30}');
    expect(result).toEqual({ name: "Alice", age: 30 });
  });

  it("should return {} for invalid JSON string", () => {
    const result = safeParseJSON("not valid json");
    expect(result).toEqual({});
  });

  it("should return {} for undefined input", () => {
    const result = safeParseJSON(undefined);
    expect(result).toEqual({});
  });

  it("should return {} for empty string", () => {
    const result = safeParseJSON("");
    expect(result).toEqual({});
  });

  it("should parse JSON array", () => {
    const result = safeParseJSON("[1,2,3]");
    expect(result).toEqual([1, 2, 3]);
  });
});
