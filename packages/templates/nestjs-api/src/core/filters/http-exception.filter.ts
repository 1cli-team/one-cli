import {
  type ArgumentsHost,
  Catch,
  type ExceptionFilter,
  HttpException,
  HttpStatus,
  Logger,
} from "@nestjs/common";
import type { Request, Response } from "express";
import { ErrorCode, getErrorMessage } from "../constants/error-code";
import { BusinessException } from "../exceptions/business.exception";

interface ErrorResponse {
  code: number;
  message: string;
  data: null;
  timestamp: string;
  path: string;
}

@Catch()
export class HttpExceptionFilter implements ExceptionFilter {
  private readonly logger = new Logger(HttpExceptionFilter.name);

  catch(exception: unknown, host: ArgumentsHost) {
    const ctx = host.switchToHttp();
    const response = ctx.getResponse<Response>();
    const request = ctx.getRequest<Request>();

    let status = HttpStatus.INTERNAL_SERVER_ERROR;
    let message = getErrorMessage(ErrorCode.INTERNAL_SERVER_ERROR);
    let code = ErrorCode.INTERNAL_SERVER_ERROR;

    // 处理 BusinessException
    if (exception instanceof BusinessException) {
      const exceptionResponse = exception.getResponse() as any;
      status = exception.getStatus();
      code = exceptionResponse.code;
      message = exceptionResponse.message;
    }
    // 处理通用 HttpException
    else if (exception instanceof HttpException) {
      const exceptionResponse = exception.getResponse();
      status = exception.getStatus();

      if (typeof exceptionResponse === "object") {
        const responseData = exceptionResponse as any;
        message = responseData.message || this.getDefaultHttpMessage(status);
        code = responseData.code || this.getDefaultErrorCode(status);

        // 处理验证错误数组
        if (Array.isArray(responseData.message)) {
          message = responseData.message.join(", ");
        }
      } else {
        message = exceptionResponse as string;
        code = this.getDefaultErrorCode(status);
      }
    }
    // 处理其他错误
    else if (exception instanceof Error) {
      message = exception.message;
      this.logger.error(
        `Unhandled exception: ${exception.message}`,
        exception.stack,
      );
    }

    // 记录错误日志
    this.logError(exception, request, status, code);

    const errorResponse: ErrorResponse = {
      code,
      message,
      data: null,
      timestamp: new Date().toISOString(),
      path: request.path,
    };

    response.status(status).json(errorResponse);
  }

  private getDefaultErrorCode(httpStatus: HttpStatus): ErrorCode {
    switch (httpStatus) {
      case HttpStatus.BAD_REQUEST:
        return ErrorCode.INVALID_PARAMS;
      case HttpStatus.UNAUTHORIZED:
        return ErrorCode.UNAUTHORIZED;
      case HttpStatus.FORBIDDEN:
        return ErrorCode.FORBIDDEN;
      case HttpStatus.NOT_FOUND:
        return ErrorCode.NOT_FOUND;
      case HttpStatus.METHOD_NOT_ALLOWED:
        return ErrorCode.METHOD_NOT_ALLOWED;
      case HttpStatus.TOO_MANY_REQUESTS:
        return ErrorCode.RATE_LIMITED;
      default:
        return ErrorCode.INTERNAL_SERVER_ERROR;
    }
  }

  private getDefaultHttpMessage(httpStatus: HttpStatus): string {
    const errorCode = this.getDefaultErrorCode(httpStatus);
    return getErrorMessage(errorCode);
  }

  private logError(
    exception: unknown,
    request: Request,
    status: HttpStatus,
    code: ErrorCode,
  ) {
    const message =
      exception instanceof Error ? exception.message : "Unknown error";
    const stack = exception instanceof Error ? exception.stack : "";

    const logData = {
      method: request.method,
      path: request.path,
      userAgent: request.get("user-agent"),
      ip: request.ip,
      status,
      code,
      message,
    };

    if (status >= 500) {
      this.logger.error("Internal server error", { ...logData, stack });
    } else if (status >= 400) {
      this.logger.warn("Client error", logData);
    }
  }
}
