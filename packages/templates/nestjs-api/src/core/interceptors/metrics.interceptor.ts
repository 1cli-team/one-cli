import {
  CallHandler,
  ExecutionContext,
  Injectable,
  NestInterceptor,
} from "@nestjs/common";
import { InjectMetric } from "@willsoto/nestjs-prometheus";
import { Counter, Histogram } from "prom-client";
import { Observable, throwError } from "rxjs";
import { catchError, tap } from "rxjs/operators";

@Injectable()
export class MetricsInterceptor implements NestInterceptor {
  constructor(
    @InjectMetric("http_requests_total") public counter: Counter<string>,
    @InjectMetric("http_request_duration_seconds")
    public histogram: Histogram<string>,
  ) {}

  intercept(context: ExecutionContext, next: CallHandler): Observable<unknown> {
    const req = context.switchToHttp().getRequest();
    const res = context.switchToHttp().getResponse();

    const method = req.method;
    const route = req.route?.path || req.url;

    const timer = this.histogram.startTimer({ method, route });

    return next.handle().pipe(
      tap(() => {
        const status = res.statusCode.toString();
        this.counter.inc({ method, route, status });
        timer({ method, route, status });
      }),
      catchError((error) => {
        const status = error.status?.toString() || "500";
        this.counter.inc({ method, route, status });
        timer({ method, route, status });
        return throwError(() => error);
      }),
    );
  }
}
