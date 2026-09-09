// 配置加载策略（One CLI 推荐）：
//  1. 用 @nestjs/config 的 registerAs 把 process.env.* 映射成嵌套
//     namespace（jwt / server / database / app），ConfigService 消费。
//  2. 通过 `one env set <KEY>=<value>` 设置的 secret 会在 `one run` /
//     `one dev` 启动时被自动注入为环境变量，无须额外文件渲染。
//
//     one env set DATABASE_URL=postgres://prod/db -p api-nest
//     one env set JWT_SECRET=real-secret           -p api-nest
//
//     ConfigService 里：
//       config.get('database.url')  ←  process.env.DATABASE_URL
//       config.get('jwt.secret')    ←  process.env.JWT_SECRET
//
//  3. 默认值在下方各 registerAs 工厂里兜底；生产环境缺关键 secret
//     仍直接抛错（NODE_ENV=production），非生产环境则生成一次性随机
//     fallback 以便首跑即可启动。
//
// 因此**不要**用模板渲染 / 文件生成方式补 config，运行时 env 已经覆盖。
import { Logger } from "@nestjs/common";
import { registerAs } from "@nestjs/config";
import { randomBytes } from "node:crypto";

const configLogger = new Logger("Config");

export const jwtConfig = registerAs("jwt", () => {
  let secret = process.env.JWT_SECRET;
  if (!secret) {
    if (process.env.NODE_ENV === "production") {
      throw new Error(
        "JWT_SECRET environment variable is required in production",
      );
    }
    secret = randomBytes(32).toString("hex");
    configLogger.warn(
      "JWT_SECRET 未设置；已生成一次性随机 secret（每次启动都会变，仅用于本地开发）。" +
        " 跑 `one env set JWT_SECRET=<value>` 设置稳定值。",
    );
  }
  return {
    secret,
    expiresIn: process.env.JWT_EXPIRES_IN || "30d",
  };
});

export const serverConfig = registerAs("server", () => ({
  port: parseInt(process.env.PORT || "3000", 10),
}));

export const databaseConfig = registerAs("database", () => ({
  url: process.env.DATABASE_URL,
}));

export const appConfig = registerAs("app", () => ({
  responseInterceptorExcludePaths: [
    "stream",
    "yungouos/callback",
    "/metrics",
    "/health",
  ],
  authGuardExcludePaths: ["/metrics", "/health"],
}));
