import { HttpException, HttpStatus } from "@nestjs/common";
import { ErrorCode, getErrorMessage } from "../constants/error-code";

export class BusinessException extends HttpException {
  constructor(
    errorCode: ErrorCode,
    message?: string,
    httpStatus: HttpStatus = HttpStatus.BAD_REQUEST,
  ) {
    const errorMessage = message || getErrorMessage(errorCode);

    super(
      {
        code: errorCode,
        message: errorMessage,
      },
      httpStatus,
    );
  }

  static unauthorized(
    errorCode: ErrorCode = ErrorCode.UNAUTHORIZED,
    message?: string,
  ) {
    return new BusinessException(errorCode, message, HttpStatus.UNAUTHORIZED);
  }

  static forbidden(
    errorCode: ErrorCode = ErrorCode.FORBIDDEN,
    message?: string,
  ) {
    return new BusinessException(errorCode, message, HttpStatus.FORBIDDEN);
  }

  static notFound(
    errorCode: ErrorCode = ErrorCode.NOT_FOUND,
    message?: string,
  ) {
    return new BusinessException(errorCode, message, HttpStatus.NOT_FOUND);
  }

  static badRequest(
    errorCode: ErrorCode = ErrorCode.INVALID_PARAMS,
    message?: string,
  ) {
    return new BusinessException(errorCode, message, HttpStatus.BAD_REQUEST);
  }

  static internalServerError(
    errorCode: ErrorCode = ErrorCode.INTERNAL_SERVER_ERROR,
    message?: string,
  ) {
    return new BusinessException(
      errorCode,
      message,
      HttpStatus.INTERNAL_SERVER_ERROR,
    );
  }

  static tooManyRequests(
    errorCode: ErrorCode = ErrorCode.RATE_LIMITED,
    message?: string,
  ) {
    return new BusinessException(
      errorCode,
      message,
      HttpStatus.TOO_MANY_REQUESTS,
    );
  }

  // 认证相关静态方法
  static authTokenMissing(message?: string) {
    return BusinessException.unauthorized(
      ErrorCode.AUTH_TOKEN_MISSING,
      message,
    );
  }

  static authTokenInvalid(message?: string) {
    return BusinessException.unauthorized(
      ErrorCode.AUTH_TOKEN_INVALID,
      message,
    );
  }

  static authTokenExpired(message?: string) {
    return BusinessException.unauthorized(
      ErrorCode.AUTH_TOKEN_EXPIRED,
      message,
    );
  }

  static authLoginFailed(message?: string) {
    return BusinessException.unauthorized(ErrorCode.AUTH_LOGIN_FAILED, message);
  }

  // 用户相关静态方法
  static userNotFound(message?: string) {
    return BusinessException.notFound(ErrorCode.USER_NOT_FOUND, message);
  }

  static userAlreadyExists(message?: string) {
    return BusinessException.badRequest(ErrorCode.USER_ALREADY_EXISTS, message);
  }

  static userNameAlreadyExists(message?: string) {
    return BusinessException.badRequest(
      ErrorCode.USER_NAME_ALREADY_EXISTS,
      message,
    );
  }

  static userEmailAlreadyExists(message?: string) {
    return BusinessException.badRequest(
      ErrorCode.USER_EMAIL_ALREADY_EXISTS,
      message,
    );
  }

  static userPermissionDenied(message?: string) {
    return BusinessException.forbidden(
      ErrorCode.USER_PERMISSION_DENIED,
      message,
    );
  }

  // 业务逻辑相关静态方法
  static businessRuleViolation(message?: string) {
    return BusinessException.badRequest(
      ErrorCode.BUSINESS_RULE_VIOLATION,
      message,
    );
  }

  static resourceConflict(message?: string) {
    return new BusinessException(
      ErrorCode.RESOURCE_CONFLICT,
      message,
      HttpStatus.CONFLICT,
    );
  }

  static operationNotAllowed(message?: string) {
    return BusinessException.forbidden(
      ErrorCode.OPERATION_NOT_ALLOWED,
      message,
    );
  }

  static quotaExceeded(message?: string) {
    return new BusinessException(
      ErrorCode.QUOTA_EXCEEDED,
      message,
      HttpStatus.PAYLOAD_TOO_LARGE,
    );
  }
}
