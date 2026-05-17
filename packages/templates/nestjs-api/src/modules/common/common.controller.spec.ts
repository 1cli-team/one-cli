jest.mock("@/shared/utils", () => ({
  logger: {
    info: jest.fn(),
    error: jest.fn(),
    warn: jest.fn(),
    debug: jest.fn(),
  },
}));

import { Test } from "@nestjs/testing";
import { CommonController } from "./common.controller";
import { CommonService } from "./common.service";

describe("CommonController", () => {
  let controller: CommonController;
  let commonService: CommonService;

  beforeEach(async () => {
    const module = await Test.createTestingModule({
      controllers: [CommonController],
      providers: [
        {
          provide: CommonService,
          useValue: {
            getHello: jest.fn().mockReturnValue("Hello Alice!"),
          },
        },
      ],
    }).compile();

    controller = module.get(CommonController);
    commonService = module.get(CommonService);
  });

  describe("getHello", () => {
    it("should call commonService.getHello with user", () => {
      const result = controller.getHello({ user: "Alice" });

      expect(commonService.getHello).toHaveBeenCalledWith("Alice");
      expect(result).toBe("Hello Alice!");
    });
  });

  describe("test", () => {
    it('should return "this endpoint is public"', () => {
      expect(controller.test()).toBe("this endpoint is public");
    });
  });
});
