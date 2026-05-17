jest.mock("@/shared/utils", () => ({
  logger: {
    info: jest.fn(),
    error: jest.fn(),
    warn: jest.fn(),
    debug: jest.fn(),
  },
}));

import type { CallHandler, ExecutionContext } from "@nestjs/common";
import { of } from "rxjs";
import { lastValueFrom } from "rxjs";
import { logger } from "@/shared/utils";
import { LoggingInterceptor } from "./logging.interceptor";

describe("LoggingInterceptor", () => {
  let interceptor: LoggingInterceptor;

  beforeEach(() => {
    interceptor = new LoggingInterceptor();
    jest.clearAllMocks();
  });

  function createMockContext(
    method: string,
    url: string,
    query: any = {},
    body: any = {},
    statusCode = 200,
  ): ExecutionContext {
    return {
      switchToHttp: () => ({
        getRequest: () => ({ method, url, query, body }),
        getResponse: () => ({ statusCode }),
      }),
    } as unknown as ExecutionContext;
  }

  function createMockCallHandler(data: any): CallHandler {
    return { handle: () => of(data) };
  }

  it("should log request info on intercept", async () => {
    const context = createMockContext(
      "GET",
      "/api/v1/users",
      { page: "1" },
      {},
    );
    const handler = createMockCallHandler("result");

    await lastValueFrom(interceptor.intercept(context, handler));

    expect(logger.info).toHaveBeenCalledWith(
      expect.stringContaining("GET /api/v1/users"),
    );
    expect(logger.info).toHaveBeenCalledWith(
      expect.stringContaining('"page":"1"'),
    );
  });

  it("should log response status code", async () => {
    const context = createMockContext(
      "POST",
      "/api/v1/users",
      {},
      { name: "Alice" },
      201,
    );
    const handler = createMockCallHandler("result");

    await lastValueFrom(interceptor.intercept(context, handler));

    expect(logger.info).toHaveBeenCalledWith(
      expect.stringContaining("Status: 201"),
    );
  });

  it("should pass through the handler result", async () => {
    const context = createMockContext("GET", "/test");
    const handler = createMockCallHandler({ data: "test" });

    const result = await lastValueFrom(interceptor.intercept(context, handler));

    expect(result).toEqual({ data: "test" });
  });

  it("should sanitize sensitive fields in body", async () => {
    const context = createMockContext(
      "POST",
      "/api/v1/auth/login",
      {},
      {
        username: "alice",
        password: "secret123",
        token: "jwt-token",
      },
    );
    const handler = createMockCallHandler("ok");

    await lastValueFrom(interceptor.intercept(context, handler));

    const logCall = (logger.info as jest.Mock).mock.calls[0][0];
    expect(logCall).toContain('"username":"alice"');
    expect(logCall).toContain('"password":"***"');
    expect(logCall).toContain('"token":"***"');
    expect(logCall).not.toContain("secret123");
    expect(logCall).not.toContain("jwt-token");
  });

  it("should sanitize sensitive fields in query", async () => {
    const context = createMockContext("GET", "/api/v1/test", {
      secret: "my-key",
      page: "1",
    });
    const handler = createMockCallHandler("ok");

    await lastValueFrom(interceptor.intercept(context, handler));

    const logCall = (logger.info as jest.Mock).mock.calls[0][0];
    expect(logCall).toContain('"secret":"***"');
    expect(logCall).toContain('"page":"1"');
    expect(logCall).not.toContain("my-key");
  });
});
