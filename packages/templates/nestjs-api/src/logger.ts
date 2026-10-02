import type { LoggerService } from "@nestjs/common";
import pino from "pino";

const output = pino();
export const logger: LoggerService = {
  log: (message: unknown, ...context: unknown[]) =>
    output.info({ context }, String(message)),
  error: (message: unknown, ...context: unknown[]) =>
    output.error({ context }, String(message)),
  warn: (message: unknown, ...context: unknown[]) =>
    output.warn({ context }, String(message)),
  debug: (message: unknown, ...context: unknown[]) =>
    output.debug({ context }, String(message)),
  verbose: (message: unknown, ...context: unknown[]) =>
    output.trace({ context }, String(message)),
  fatal: (message: unknown, ...context: unknown[]) =>
    output.fatal({ context }, String(message)),
};
