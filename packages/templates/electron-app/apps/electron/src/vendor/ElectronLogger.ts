import { mkdirSync } from "node:fs";
import path from "node:path";
import dayjs from "dayjs";
import logger from "electron-log";
import { appName, workspace } from "../utils";

const normalizeArgs = (args: unknown[]): unknown[] =>
  args.map((arg) => (arg instanceof Error ? (arg.stack ?? arg.message) : arg));

export function createElectronLogger() {
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

  return {
    logger,
    info: (...args: unknown[]) => logger.info(...normalizeArgs(args)),
    warn: (...args: unknown[]) => logger.warn(...normalizeArgs(args)),
    error: (...args: unknown[]) => logger.error(...normalizeArgs(args)),
    debug: (...args: unknown[]) => logger.debug(...normalizeArgs(args)),
  };
}

export type ElectronLogger = ReturnType<typeof createElectronLogger>;
