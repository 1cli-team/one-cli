import { mkdirSync } from "node:fs";
import path from "node:path";
import { provide } from "@inversifyjs/binding-decorators";
import dayjs from "dayjs";
import logger, { type Logger } from "electron-log";
import { injectable } from "inversify";
import { appName, workspace } from "../utils";

// 日志：文件 + 控制台双通道
// 文件按天切分，单文件 10MB 上限
// Error 对象自动提取 stack，避免只打出 "[object Object]"
@injectable()
@provide()
export default class ElectronLogger {
  logger: Logger;

  constructor() {
    const datetime = dayjs().format("YYYY-MM-DD");
    const logPath = path.resolve(workspace, `logs/${datetime}-${appName}.log`);
    mkdirSync(path.dirname(logPath), { recursive: true });

    logger.transports.file.level = "info";
    logger.transports.file.resolvePathFn = () => logPath;
    logger.transports.file.format =
      "[{y}-{m}-{d} {h}:{i}:{s}.{ms}] [{level}] {text}";
    logger.transports.file.maxSize = 1024 * 1024 * 10;

    logger.transports.console.level =
      process.env.NODE_ENV === "development" ? "debug" : "info";
    logger.transports.console.format = "{h}:{i}:{s} [{level}] {text}";

    this.logger = logger;
  }

  info(...args: unknown[]) {
    return this.logger.info(...this.normalizeArgs(args));
  }
  warn(...args: unknown[]) {
    return this.logger.warn(...this.normalizeArgs(args));
  }
  error(...args: unknown[]) {
    return this.logger.error(...this.normalizeArgs(args));
  }
  debug(...args: unknown[]) {
    return this.logger.debug(...this.normalizeArgs(args));
  }

  private normalizeArgs(args: unknown[]): unknown[] {
    return args.map((arg) => {
      if (arg instanceof Error) return arg.stack ?? arg.message;
      return arg;
    });
  }
}
