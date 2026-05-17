import { HttpException, HttpStatus } from "@nestjs/common";
import type { ArgumentsHost } from "@nestjs/common";
import { ErrorCode } from "../constants/error-code";
import { BusinessException } from "../exceptions/business.exception";
import { HttpExceptionFilter } from "./http-exception.filter";

describe("HttpExceptionFilter", () => {
  let filter: HttpExceptionFilter;
  let mockResponse: any;
  let mockRequest: any;
  let mockHost: ArgumentsHost;

  beforeEach(() => {
    filter = new HttpExceptionFilter();
    mockResponse = {
      status: jest.fn().mockReturnThis(),
      json: jest.fn(),
    };
    mockRequest = {
      path: "/test",
      method: "GET",
      get: jest.fn().mockReturnValue("test-agent"),
      ip: "127.0.0.1",
    };
    mockHost = {
      switchToHttp: () => ({
        getResponse: () => mockResponse,
        getRequest: () => mockRequest,
      }),
    } as unknown as ArgumentsHost;
  });

  it("should handle BusinessException", () => {
    const exception = BusinessException.notFound(
      ErrorCode.USER_NOT_FOUND,
      "user not found",
    );

    filter.catch(exception, mockHost);

    expect(mockResponse.status).toHaveBeenCalledWith(HttpStatus.NOT_FOUND);
    const responseBody = mockResponse.json.mock.calls[0][0];
    expect(responseBody.code).toBe(ErrorCode.USER_NOT_FOUND);
    expect(responseBody.message).toBe("user not found");
    expect(responseBody.data).toBeNull();
    expect(responseBody.path).toBe("/test");
    expect(responseBody.timestamp).toBeDefined();
  });

  it("should handle HttpException with object response", () => {
    const exception = new HttpException(
      { message: "bad request", statusCode: 400 },
      HttpStatus.BAD_REQUEST,
    );

    filter.catch(exception, mockHost);

    expect(mockResponse.status).toHaveBeenCalledWith(HttpStatus.BAD_REQUEST);
    const responseBody = mockResponse.json.mock.calls[0][0];
    expect(responseBody.message).toBe("bad request");
    expect(responseBody.code).toBe(ErrorCode.INVALID_PARAMS);
  });

  it("should handle HttpException with validation error array", () => {
    const exception = new HttpException(
      { message: ["field1 is required", "field2 must be string"] },
      HttpStatus.BAD_REQUEST,
    );

    filter.catch(exception, mockHost);

    const responseBody = mockResponse.json.mock.calls[0][0];
    expect(responseBody.message).toBe(
      "field1 is required, field2 must be string",
    );
  });

  it("should handle HttpException with string response", () => {
    const exception = new HttpException("Not Found", HttpStatus.NOT_FOUND);

    filter.catch(exception, mockHost);

    expect(mockResponse.status).toHaveBeenCalledWith(HttpStatus.NOT_FOUND);
    const responseBody = mockResponse.json.mock.calls[0][0];
    expect(responseBody.message).toBe("Not Found");
    expect(responseBody.code).toBe(ErrorCode.NOT_FOUND);
  });

  it("should handle generic Error", () => {
    const exception = new Error("something went wrong");

    filter.catch(exception, mockHost);

    expect(mockResponse.status).toHaveBeenCalledWith(
      HttpStatus.INTERNAL_SERVER_ERROR,
    );
    const responseBody = mockResponse.json.mock.calls[0][0];
    expect(responseBody.message).toBe("something went wrong");
    expect(responseBody.code).toBe(ErrorCode.INTERNAL_SERVER_ERROR);
  });

  it("should handle unknown exception", () => {
    filter.catch("unknown error", mockHost);

    expect(mockResponse.status).toHaveBeenCalledWith(
      HttpStatus.INTERNAL_SERVER_ERROR,
    );
    const responseBody = mockResponse.json.mock.calls[0][0];
    expect(responseBody.code).toBe(ErrorCode.INTERNAL_SERVER_ERROR);
  });

  it("should include timestamp and path in every response", () => {
    const exception = new HttpException("test", HttpStatus.BAD_REQUEST);

    filter.catch(exception, mockHost);

    const responseBody = mockResponse.json.mock.calls[0][0];
    expect(responseBody.timestamp).toBeDefined();
    expect(responseBody.path).toBe("/test");
    expect(responseBody.data).toBeNull();
  });

  it("should map HttpStatus to correct ErrorCode", () => {
    const cases = [
      { status: HttpStatus.UNAUTHORIZED, code: ErrorCode.UNAUTHORIZED },
      { status: HttpStatus.FORBIDDEN, code: ErrorCode.FORBIDDEN },
      { status: HttpStatus.TOO_MANY_REQUESTS, code: ErrorCode.RATE_LIMITED },
    ];

    for (const { status, code } of cases) {
      const exception = new HttpException("test", status);
      filter.catch(exception, mockHost);
      const responseBody = mockResponse.json.mock.calls.at(-1)[0];
      expect(responseBody.code).toBe(code);
    }
  });
});
