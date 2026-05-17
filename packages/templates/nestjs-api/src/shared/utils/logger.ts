import pino from "pino";
import { join } from "path";

const logsDir = join(process.cwd(), "logs");

const transport = pino.transport({
  targets: [
    {
      // 控制台输出，使用 pino-pretty 美化
      target: "pino-pretty",
      options: {
        colorize: true,
        levelFirst: true,
        translateTime: "yyyy-mm-dd HH:MM:ss",
        messageFormat: "{level} {msg}",
      },
    },
    {
      // 错误日志 - 按日期切分，保留 30 天
      target: "pino-roll",
      level: "error",
      options: {
        file: join(logsDir, "error", "error"),
        frequency: "daily",
        dateFormat: "yyyy-MM-dd",
        mkdir: true,
        limit: { count: 30 },
      },
    },
    {
      // 警告日志 - 按日期切分，保留 30 天
      target: "pino-roll",
      level: "warn",
      options: {
        file: join(logsDir, "warn", "warn"),
        frequency: "daily",
        dateFormat: "yyyy-MM-dd",
        mkdir: true,
        limit: { count: 30 },
      },
    },
    {
      // 信息日志 - 按日期切分，保留 30 天
      target: "pino-roll",
      level: "info",
      options: {
        file: join(logsDir, "info", "info"),
        frequency: "daily",
        dateFormat: "yyyy-MM-dd",
        mkdir: true,
        limit: { count: 30 },
      },
    },
    {
      // 调试日志 - 按日期切分，保留 7 天
      target: "pino-roll",
      level: "debug",
      options: {
        file: join(logsDir, "debug", "debug"),
        frequency: "daily",
        dateFormat: "yyyy-MM-dd",
        mkdir: true,
        limit: { count: 7 },
      },
    },
  ],
});

export const logger = pino(
  {
    level: process.env.NODE_ENV === "production" ? "info" : "debug",
  },
  transport,
);

// 导出常用的日志方法
export const logInfo = (msg: string, ...args: any[]) =>
  logger.info(msg, ...args);
export const logError = (msg: string, ...args: any[]) =>
  logger.error(msg, ...args);
export const logWarn = (msg: string, ...args: any[]) =>
  logger.warn(msg, ...args);
export const logDebug = (msg: string, ...args: any[]) =>
  logger.debug(msg, ...args);
