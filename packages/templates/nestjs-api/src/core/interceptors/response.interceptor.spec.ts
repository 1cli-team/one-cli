import type { CallHandler, ExecutionContext } from "@nestjs/common";
import { ConfigService } from "@nestjs/config";
import { Test } from "@nestjs/testing";
import { lastValueFrom, of } from "rxjs";
import { ResponseInterceptor } from "./response.interceptor";

describe("ResponseInterceptor", () => {
  let interceptor: ResponseInterceptor<any>;
  let configService: ConfigService;

  beforeEach(async () => {
    const module = await Test.createTestingModule({
      providers: [
        ResponseInterceptor,
        {
          provide: ConfigService,
          useValue: {
            get: jest.fn((key: string) => {
              if (key === "app.responseInterceptorExcludePaths") {
                return ["/metrics", "/health", "stream"];
              }
              return undefined;
            }),
          },
        },
      ],
    }).compile();

    interceptor = module.get(ResponseInterceptor);
    configService = module.get(ConfigService);
  });

  function createMockContext(path: string): ExecutionContext {
    return {
      switchToHttp: () => ({
        getRequest: () => ({ path }),
      }),
    } as unknown as ExecutionContext;
  }

  function createMockCallHandler(data: any): CallHandler {
    return { handle: () => of(data) };
  }

  it("should wrap response data in standard format", async () => {
    const context = createMockContext("/api/v1/users");
    const handler = createMockCallHandler({ id: "1", name: "Alice" });

    const result = await lastValueFrom(interceptor.intercept(context, handler));

    expect(result).toEqual({
      code: 0,
      message: "success",
      data: { id: "1", name: "Alice" },
    });
  });

  it("should pass through for exact exclude path", async () => {
    const context = createMockContext("/metrics");
    const testData = { metric: "value" };
    const handler = createMockCallHandler(testData);

    const result = await lastValueFrom(interceptor.intercept(context, handler));

    expect(result).toEqual(testData);
  });

  it("should pass through for substring exclude path", async () => {
    const context = createMockContext("/api/v1/stream/data");
    const testData = "raw stream data";
    const handler = createMockCallHandler(testData);

    const result = await lastValueFrom(interceptor.intercept(context, handler));

    expect(result).toBe(testData);
  });

  it("should use empty array when config returns undefined", async () => {
    jest.spyOn(configService, "get").mockReturnValue(undefined);

    const context = createMockContext("/metrics");
    const handler = createMockCallHandler("data");

    const result = await lastValueFrom(interceptor.intercept(context, handler));

    expect(result).toEqual({
      code: 0,
      message: "success",
      data: "data",
    });
  });

  it("should wrap null data", async () => {
    const context = createMockContext("/api/v1/test");
    const handler = createMockCallHandler(null);

    const result = await lastValueFrom(interceptor.intercept(context, handler));

    expect(result).toEqual({
      code: 0,
      message: "success",
      data: null,
    });
  });
});
