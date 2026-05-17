import { CommonService } from "./common.service";

describe("CommonService", () => {
  let service: CommonService;

  beforeEach(() => {
    service = new CommonService();
  });

  describe("getHello", () => {
    it('should return "Hello World!" when called with "World"', () => {
      expect(service.getHello("World")).toBe("Hello World!");
    });

    it('should return "Hello !" when called with empty string', () => {
      expect(service.getHello("")).toBe("Hello !");
    });

    it("should correctly interpolate the user parameter", () => {
      expect(service.getHello("Alice")).toBe("Hello Alice!");
    });
  });
});
