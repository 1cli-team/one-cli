import { UnauthorizedException } from "@nestjs/common";
import { JwtService } from "@nestjs/jwt";
import { Test } from "@nestjs/testing";
import { AuthService } from "./auth.service";

describe("AuthService", () => {
  let authService: AuthService;
  let jwtService: JwtService;

  beforeEach(async () => {
    const module = await Test.createTestingModule({
      providers: [
        AuthService,
        {
          provide: JwtService,
          useValue: {
            sign: jest.fn().mockReturnValue("mock-token"),
            verify: jest.fn().mockReturnValue({ userId: "test-user" }),
          },
        },
      ],
    }).compile();

    authService = module.get(AuthService);
    jwtService = module.get(JwtService);
  });

  describe("getAccessToken", () => {
    it("should sign a token with userId from code", async () => {
      const result = await authService.getAccessToken("auth-code-123");

      expect(jwtService.sign).toHaveBeenCalledWith({ userId: "auth-code-123" });
      expect(result).toBe("mock-token");
    });
  });

  describe("refreshAccessToken", () => {
    it("should verify with ignoreExpiration and re-sign with clean payload", async () => {
      jest.spyOn(jwtService, "verify").mockReturnValue({
        userId: "test-user",
        iat: 1000,
        exp: 2000,
      } as any);

      const result = await authService.refreshAccessToken("old-token");

      expect(jwtService.verify).toHaveBeenCalledWith("old-token", {
        ignoreExpiration: true,
      });
      expect(jwtService.sign).toHaveBeenCalledWith({ userId: "test-user" });
      expect(result).toBe("mock-token");
    });

    it("should throw UnauthorizedException when verify throws", async () => {
      jest.spyOn(jwtService, "verify").mockImplementation(() => {
        throw new Error("invalid token");
      });

      await expect(authService.refreshAccessToken("bad-token")).rejects.toThrow(
        UnauthorizedException,
      );
    });
  });

  describe("verifyAccessToken", () => {
    it("should verify and return the payload", () => {
      const result = authService.verifyAccessToken("valid-token");

      expect(jwtService.verify).toHaveBeenCalledWith("valid-token");
      expect(result).toEqual({ userId: "test-user" });
    });
  });
});
