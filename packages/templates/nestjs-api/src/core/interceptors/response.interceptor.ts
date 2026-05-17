// 统一接口响应格式拦截器，将响应数据包装为 { code, message, data } 结构。
// 通过配置 app.responseInterceptorExcludePaths 可排除特定路径（如 stream、metrics 等）。
import {
  type CallHandler,
  type ExecutionContext,
  Injectable,
  type NestInterceptor,
} from "@nestjs/common";
import { ConfigService } from "@nestjs/config";
import type { Observable } from "rxjs";
import { map } from "rxjs/operators";

@Injectable()
export class ResponseInterceptor<T> implements NestInterceptor<T, any> {
  constructor(private readonly configService: ConfigService) {}

  intercept(context: ExecutionContext, next: CallHandler<T>): Observable<any> {
    const request = context.switchToHttp().getRequest();
    const excludePaths =
      this.configService.get<string[]>("app.responseInterceptorExcludePaths") ||
      [];

    const shouldExclude = excludePaths.some((excludePath) => {
      if (excludePath.startsWith("/")) {
        return request.path === excludePath;
      }
      // 按路径段匹配，避免 "stream" 意外匹配 "/upstream"
      return request.path.split("/").includes(excludePath);
    });

    if (shouldExclude) {
      return next.handle();
    }

    return next.handle().pipe(
      map((data) => ({
        code: 0,
        message: "success",
        data,
      })),
    );
  }
}
