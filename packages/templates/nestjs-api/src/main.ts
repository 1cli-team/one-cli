import "reflect-metadata";
import { ValidationPipe } from "@nestjs/common";
import { NestFactory } from "@nestjs/core";
import { DocumentBuilder, SwaggerModule } from "@nestjs/swagger";
import { AppModule } from "./app.module";
import { logger } from "./logger";

async function bootstrap() {
  const app = await NestFactory.create(AppModule, { logger });
  const origins = (
    process.env.ALLOWED_ORIGINS ?? "http://localhost:3000,http://localhost:5173"
  )
    .split(",")
    .map((origin) => origin.trim())
    .filter(Boolean);
  app.enableCors({ origin: origins });
  app.useGlobalPipes(new ValidationPipe({ whitelist: true, transform: true }));
  app.enableShutdownHooks();
  const config = new DocumentBuilder()
    .setTitle("NestJS API")
    .setDescription("Application information and health / 服务信息与健康检查")
    .setVersion("0.1.0")
    .build();
  SwaggerModule.setup(
    "api/docs",
    app,
    SwaggerModule.createDocument(app, config),
  );
  await app.listen(Number(process.env.PORT ?? 3000));
}

void bootstrap();
