import { HttpStatus } from "@nestjs/common";
import { ErrorCode, getErrorMessage } from "../constants/error-code";
import { BusinessException } from "./business.exception";

describe("BusinessException", () => {
  describe("constructor", () => {
    it("should create with errorCode, message and httpStatus", () => {
      const ex = new BusinessException(
        ErrorCode.INVALID_PARAMS,
        "custom message",
        HttpStatus.UNPROCESSABLE_ENTITY,
      );
      expect(ex.getStatus()).toBe(HttpStatus.UNPROCESSABLE_ENTITY);
      const response = ex.getResponse() as any;
      expect(response.code).toBe(ErrorCode.INVALID_PARAMS);
      expect(response.message).toBe("custom message");
    });

    it("should use default message from getErrorMessage when message is not provided", () => {
      const ex = new BusinessException(ErrorCode.INVALID_PARAMS);
      const response = ex.getResponse() as any;
      expect(response.message).toBe(getErrorMessage(ErrorCode.INVALID_PARAMS));
    });

    it("should default to HttpStatus.BAD_REQUEST when httpStatus is not provided", () => {
      const ex = new BusinessException(ErrorCode.INVALID_PARAMS);
      expect(ex.getStatus()).toBe(HttpStatus.BAD_REQUEST);
    });
  });

  describe("static factory methods", () => {
    it("unauthorized() should create with UNAUTHORIZED status", () => {
      const ex = BusinessException.unauthorized();
      expect(ex.getStatus()).toBe(HttpStatus.UNAUTHORIZED);
      const response = ex.getResponse() as any;
      expect(response.code).toBe(ErrorCode.UNAUTHORIZED);
    });

    it("forbidden() should create with FORBIDDEN status", () => {
      const ex = BusinessException.forbidden();
      expect(ex.getStatus()).toBe(HttpStatus.FORBIDDEN);
      const response = ex.getResponse() as any;
      expect(response.code).toBe(ErrorCode.FORBIDDEN);
    });

    it("notFound() should create with NOT_FOUND status", () => {
      const ex = BusinessException.notFound();
      expect(ex.getStatus()).toBe(HttpStatus.NOT_FOUND);
      const response = ex.getResponse() as any;
      expect(response.code).toBe(ErrorCode.NOT_FOUND);
    });

    it("badRequest() should create with BAD_REQUEST status", () => {
      const ex = BusinessException.badRequest();
      expect(ex.getStatus()).toBe(HttpStatus.BAD_REQUEST);
      const response = ex.getResponse() as any;
      expect(response.code).toBe(ErrorCode.INVALID_PARAMS);
    });

    it("internalServerError() should create with INTERNAL_SERVER_ERROR status", () => {
      const ex = BusinessException.internalServerError();
      expect(ex.getStatus()).toBe(HttpStatus.INTERNAL_SERVER_ERROR);
      const response = ex.getResponse() as any;
      expect(response.code).toBe(ErrorCode.INTERNAL_SERVER_ERROR);
    });

    it("tooManyRequests() should create with TOO_MANY_REQUESTS status", () => {
      const ex = BusinessException.tooManyRequests();
      expect(ex.getStatus()).toBe(HttpStatus.TOO_MANY_REQUESTS);
      const response = ex.getResponse() as any;
      expect(response.code).toBe(ErrorCode.RATE_LIMITED);
    });
  });

  describe("auth static methods", () => {
    it("authTokenMissing() should use AUTH_TOKEN_MISSING code", () => {
      const ex = BusinessException.authTokenMissing();
      expect(ex.getStatus()).toBe(HttpStatus.UNAUTHORIZED);
      const response = ex.getResponse() as any;
      expect(response.code).toBe(ErrorCode.AUTH_TOKEN_MISSING);
    });

    it("authTokenInvalid() should use AUTH_TOKEN_INVALID code", () => {
      const ex = BusinessException.authTokenInvalid();
      const response = ex.getResponse() as any;
      expect(response.code).toBe(ErrorCode.AUTH_TOKEN_INVALID);
    });

    it("authTokenExpired() should use AUTH_TOKEN_EXPIRED code", () => {
      const ex = BusinessException.authTokenExpired();
      const response = ex.getResponse() as any;
      expect(response.code).toBe(ErrorCode.AUTH_TOKEN_EXPIRED);
    });

    it("authLoginFailed() should use AUTH_LOGIN_FAILED code", () => {
      const ex = BusinessException.authLoginFailed();
      const response = ex.getResponse() as any;
      expect(response.code).toBe(ErrorCode.AUTH_LOGIN_FAILED);
    });
  });

  describe("user static methods", () => {
    it("userNotFound() should use NOT_FOUND status", () => {
      const ex = BusinessException.userNotFound();
      expect(ex.getStatus()).toBe(HttpStatus.NOT_FOUND);
      const response = ex.getResponse() as any;
      expect(response.code).toBe(ErrorCode.USER_NOT_FOUND);
    });

    it("userAlreadyExists() should use BAD_REQUEST status", () => {
      const ex = BusinessException.userAlreadyExists();
      expect(ex.getStatus()).toBe(HttpStatus.BAD_REQUEST);
      const response = ex.getResponse() as any;
      expect(response.code).toBe(ErrorCode.USER_ALREADY_EXISTS);
    });

    it("userPermissionDenied() should use FORBIDDEN status", () => {
      const ex = BusinessException.userPermissionDenied();
      expect(ex.getStatus()).toBe(HttpStatus.FORBIDDEN);
      const response = ex.getResponse() as any;
      expect(response.code).toBe(ErrorCode.USER_PERMISSION_DENIED);
    });
  });

  describe("business static methods", () => {
    it("resourceConflict() should use CONFLICT status", () => {
      const ex = BusinessException.resourceConflict();
      expect(ex.getStatus()).toBe(HttpStatus.CONFLICT);
      const response = ex.getResponse() as any;
      expect(response.code).toBe(ErrorCode.RESOURCE_CONFLICT);
    });

    it("quotaExceeded() should use PAYLOAD_TOO_LARGE status", () => {
      const ex = BusinessException.quotaExceeded();
      expect(ex.getStatus()).toBe(HttpStatus.PAYLOAD_TOO_LARGE);
      const response = ex.getResponse() as any;
      expect(response.code).toBe(ErrorCode.QUOTA_EXCEEDED);
    });

    it("operationNotAllowed() should use FORBIDDEN status", () => {
      const ex = BusinessException.operationNotAllowed();
      expect(ex.getStatus()).toBe(HttpStatus.FORBIDDEN);
      const response = ex.getResponse() as any;
      expect(response.code).toBe(ErrorCode.OPERATION_NOT_ALLOWED);
    });

    it("businessRuleViolation() should use BAD_REQUEST status", () => {
      const ex = BusinessException.businessRuleViolation();
      expect(ex.getStatus()).toBe(HttpStatus.BAD_REQUEST);
      const response = ex.getResponse() as any;
      expect(response.code).toBe(ErrorCode.BUSINESS_RULE_VIOLATION);
    });
  });

  describe("custom messages", () => {
    it("should accept custom message in factory methods", () => {
      const ex = BusinessException.unauthorized(
        ErrorCode.UNAUTHORIZED,
        "custom auth error",
      );
      const response = ex.getResponse() as any;
      expect(response.message).toBe("custom auth error");
    });
  });
});
