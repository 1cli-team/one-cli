import { Module } from "@nestjs/common";
import { ConfigModule } from "@nestjs/config";
import { APP_FILTER, APP_INTERCEPTOR } from "@nestjs/core";
import * as configurations from "@/core/config/configuration";
import { HttpExceptionFilter } from "@/core/filters/http-exception.filter";
import { LoggingInterceptor } from "@/core/interceptors/logging.interceptor";
import { ResponseInterceptor } from "@/core/interceptors/response.interceptor";
import { AuthModule } from "@/modules/auth/auth.module";
import { CommonModule } from "@/modules/common/common.module";
import { HealthModule } from "@/modules/health/health.module";
import { UserModule } from "@/modules/user/user.module";
import { DrizzleModule } from "@/service/drizzle/drizzle.module";
import { AppController } from "./app.controller";

@Module({
  imports: [
    ConfigModule.forRoot({
      isGlobal: true,
      ignoreEnvFile: true,
      load: Object.values(configurations),
    }),
    DrizzleModule,
    HealthModule,
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
      useClass: LoggingInterceptor,
    },
    {
      provide: APP_FILTER,
      useClass: HttpExceptionFilter,
    },
  ],
})
export class AppModule {}
