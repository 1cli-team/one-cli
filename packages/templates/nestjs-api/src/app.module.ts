import { Module } from "@nestjs/common";
import { ConfigModule } from "@nestjs/config";
import { APP_FILTER, APP_INTERCEPTOR } from "@nestjs/core";
import { PrometheusModule } from "@willsoto/nestjs-prometheus";
import * as configurations from "@/core/config/configuration";
import { HttpExceptionFilter } from "@/core/filters/http-exception.filter";
import { LoggingInterceptor } from "@/core/interceptors/logging.interceptor";
import { MetricsInterceptor } from "@/core/interceptors/metrics.interceptor";
import { ResponseInterceptor } from "@/core/interceptors/response.interceptor";
import { AuthModule } from "@/modules/auth/auth.module";
import { CommonModule } from "@/modules/common/common.module";
import { HealthModule } from "@/modules/health/health.module";
import { MetricsModule } from "@/modules/metrics/metrics.module";
import { UserModule } from "@/modules/user/user.module";
import { DrizzleModule } from "@/service/drizzle/drizzle.module";
import { AppController } from "./app.controller";

@Module({
  imports: [
    ConfigModule.forRoot({
      isGlobal: true,
      envFilePath: ".env",
      load: Object.values(configurations),
    }),
    PrometheusModule.register({
      path: "/metrics",
      defaultLabels: {
        app: "nest-app",
      },
    }),
    DrizzleModule,
    HealthModule,
    MetricsModule,
    AuthModule,
    CommonModule,
    UserModule,
  ],
  controllers: [AppController],
  providers: [
    {
      provide: APP_INTERCEPTOR,
      useClass: ResponseInterceptor,
    },
    {
      provide: APP_INTERCEPTOR,
      useClass: MetricsInterceptor,
    },
    {
      provide: APP_INTERCEPTOR,
      useClass: LoggingInterceptor,
    },
    {
      provide: APP_FILTER,
      useClass: HttpExceptionFilter,
    },
  ],
})
export class AppModule {}
