import { UnauthorizedException } from "@nestjs/common";
import { ConfigService } from "@nestjs/config";
import { Reflector } from "@nestjs/core";
import { JwtService } from "@nestjs/jwt";
import { Test } from "@nestjs/testing";
import { AuthGuard } from "./auth.guard";

describe("AuthGuard", () => {
  let guard: AuthGuard;
  let jwtService: JwtService;
  let reflector: Reflector;
  let configService: ConfigService;

  beforeEach(async () => {
    const module = await Test.createTestingModule({
      providers: [
        AuthGuard,
        {
          provide: JwtService,
          useValue: {
            verifyAsync: jest.fn().mockResolvedValue({ userId: "test-user" }),
          },
        },
        {
          provide: Reflector,
          useValue: {
            getAllAndOverride: jest.fn().mockReturnValue(false),
          },
        },
        {
          provide: ConfigService,
          useValue: {
            get: jest.fn((key: string) => {
              if (key === "app.authGuardExcludePaths")
                return ["/metrics", "/health"];
              if (key === "jwt.secret") return "test-secret";
              return undefined;
            }),
          },
        },
      ],
    }).compile();

    guard = module.get(AuthGuard);
    jwtService = module.get(JwtService);
    reflector = module.get(Reflector);
    configService = module.get(ConfigService);
  });

  function createMockContext(path: string, authorization?: string) {
    const request: any = { path, headers: {} };
    if (authorization) {
      request.headers.authorization = authorization;
    }
    return {
      switchToHttp: () => ({ getRequest: () => request }),
      getHandler: () => jest.fn(),
      getClass: () => jest.fn(),
    } as any;
  }

  it("should allow access for exclude paths", async () => {
    const context = createMockContext("/metrics");

    const result = await guard.canActivate(context);

    expect(result).toBe(true);
  });

  it("should allow access when @Public() decorator is present", async () => {
    jest.spyOn(reflector, "getAllAndOverride").mockReturnValue(true);
    const context = createMockContext("/api/v1/auth/login");

    const result = await guard.canActivate(context);

    expect(result).toBe(true);
  });

  it("should throw UnauthorizedException when no token is present", async () => {
    const context = createMockContext("/api/v1/users");

    await expect(guard.canActivate(context)).rejects.toThrow(
      UnauthorizedException,
    );
  });

  it("should throw UnauthorizedException when Authorization header has no Bearer prefix", async () => {
    const context = createMockContext("/api/v1/users", "Basic some-token");

    await expect(guard.canActivate(context)).rejects.toThrow(
      UnauthorizedException,
    );
  });

  it("should throw UnauthorizedException when token verification fails", async () => {
    jest
      .spyOn(jwtService, "verifyAsync")
      .mockRejectedValue(new Error("invalid"));
    const context = createMockContext("/api/v1/users", "Bearer invalid-token");

    await expect(guard.canActivate(context)).rejects.toThrow(
      UnauthorizedException,
    );
  });

  it("should allow access and set request.user when token is valid", async () => {
    const context = createMockContext("/api/v1/users", "Bearer valid-token");
    const request = context.switchToHttp().getRequest();

    const result = await guard.canActivate(context);

    expect(result).toBe(true);
    expect(request.user).toEqual({ userId: "test-user" });
    expect(jwtService.verifyAsync).toHaveBeenCalledWith("valid-token", {
      secret: "test-secret",
    });
  });
});
