import {
  type CallHandler,
  type ExecutionContext,
  Injectable,
  type NestInterceptor,
} from "@nestjs/common";
import type { Observable } from "rxjs";
import { tap } from "rxjs/operators";
import { logger } from "@/shared/utils";

const SENSITIVE_FIELDS = [
  "password",
  "token",
  "secret",
  "authorization",
  "credit_card",
  "ssn",
];

function sanitize(obj: any): any {
  if (!obj || typeof obj !== "object") return obj;
  const sanitized: Record<string, any> = {};
  for (const [key, value] of Object.entries(obj)) {
    if (SENSITIVE_FIELDS.includes(key.toLowerCase())) {
      sanitized[key] = "***";
    } else {
      sanitized[key] = value;
    }
  }
  return sanitized;
}

@Injectable()
export class LoggingInterceptor implements NestInterceptor {
  intercept(context: ExecutionContext, next: CallHandler): Observable<any> {
    const req = context.switchToHttp().getRequest();
    const { method, url, body, query } = req;

    logger.info(
      `${method} ${url} - Query: ${JSON.stringify(sanitize(query))} - Body: ${JSON.stringify(sanitize(body))}`,
    );

    return next.handle().pipe(
      tap(() => {
        const res = context.switchToHttp().getResponse();
        logger.info(`${method} ${url} - Status: ${res.statusCode}`);
      }),
    );
  }
}
