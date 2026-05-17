import { Logger, ValidationPipe } from "@nestjs/common";
import { NestFactory } from "@nestjs/core";
import { DocumentBuilder, SwaggerModule } from "@nestjs/swagger";
import { AppModule } from "./app.module";

async function bootstrap() {
  // Create the application instance
  const app = await NestFactory.create(AppModule, {
    logger: ["error", "warn", "log", "debug", "verbose"],
  });

  const logger = new Logger("Bootstrap");
  const port = process.env.PORT || 3000;

  // Enable CORS with custom configuration
  const allowedOrigins = process.env.ALLOWED_ORIGINS?.split(",");
  if (!allowedOrigins) {
    logger.warn(
      "ALLOWED_ORIGINS not set, CORS will allow all origins. Do NOT use in production!",
    );
  }
  app.enableCors({
    origin: allowedOrigins || "*",
    methods: "GET,HEAD,PUT,PATCH,POST,DELETE,OPTIONS",
    credentials: true,
  });

  // Set global prefix for all routes except /metrics and /health
  app.setGlobalPrefix("api/v1", {
    exclude: ["/metrics", "/health"],
  });

  // Global validation pipe
  app.useGlobalPipes(
    new ValidationPipe({
      whitelist: true,
      transform: true,
      forbidNonWhitelisted: true,
      transformOptions: {
        enableImplicitConversion: true,
      },
    }),
  );

  // Setup Swagger documentation
  const config = new DocumentBuilder()
    .setTitle("NestJS Template API")
    .setDescription(
      "A NestJS REST API template with authentication and validation",
    )
    .setVersion("0.1.0")
    .addBearerAuth(
      {
        type: "http",
        scheme: "bearer",
        bearerFormat: "JWT",
        name: "JWT",
        description: "Enter JWT token",
        in: "header",
      },
      "JWT-auth", // This name here is important for matching up with @ApiBearerAuth() in your controller!
    )
    .addTag("auth", "Authentication endpoints")
    .addTag("common", "Common endpoints")
    .addTag("users", "User management endpoints")
    .build();

  const document = SwaggerModule.createDocument(app, config);
  SwaggerModule.setup("api/docs", app, document, {
    swaggerOptions: {
      persistAuthorization: true,
    },
  });

  // Enable graceful shutdown hooks (triggers onModuleDestroy, onApplicationShutdown, etc.)
  app.enableShutdownHooks();

  // Start the server
  await app.listen(port);
  logger.log(`Application is running on: http://localhost:${port}`);
}

bootstrap().catch((err) => {
  new Logger("Bootstrap").error("Failed to start application", err);
  process.exit(1);
});
