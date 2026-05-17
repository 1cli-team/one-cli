import { Test } from "@nestjs/testing";
import { AuthController } from "./auth.controller";
import { AuthService } from "./auth.service";

describe("AuthController", () => {
  let controller: AuthController;
  let authService: AuthService;

  beforeEach(async () => {
    const module = await Test.createTestingModule({
      controllers: [AuthController],
      providers: [
        {
          provide: AuthService,
          useValue: {
            getAccessToken: jest.fn().mockResolvedValue("mock-access-token"),
            refreshAccessToken: jest
              .fn()
              .mockResolvedValue("mock-refreshed-token"),
          },
        },
      ],
    }).compile();

    controller = module.get(AuthController);
    authService = module.get(AuthService);
  });

  describe("login", () => {
    it("should call authService.getAccessToken with code and return token", async () => {
      const result = await controller.login({ code: "auth-code-123" });

      expect(authService.getAccessToken).toHaveBeenCalledWith("auth-code-123");
      expect(result).toBe("mock-access-token");
    });
  });

  describe("refresh", () => {
    it("should call authService.refreshAccessToken with token", async () => {
      const result = await controller.refresh({ token: "old-token" });

      expect(authService.refreshAccessToken).toHaveBeenCalledWith("old-token");
      expect(result).toBe("mock-refreshed-token");
    });

    it("should propagate errors from authService", async () => {
      jest
        .spyOn(authService, "refreshAccessToken")
        .mockRejectedValue(new Error("token expired"));

      await expect(controller.refresh({ token: "bad-token" })).rejects.toThrow(
        "token expired",
      );
    });
  });
});
