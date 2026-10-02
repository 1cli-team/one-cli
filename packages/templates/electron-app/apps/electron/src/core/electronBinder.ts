import type { ControllerHandlerBinder } from "./registerControllerHandlers";
import { error, success } from "../utils/ipcResponse";
import type { ElectronLogger } from "../vendor/ElectronLogger";

export type IpcMainHandlers = {
  handle: (channel: string, listener: (...args: unknown[]) => unknown) => void;
  on: (channel: string, listener: (...args: unknown[]) => unknown) => void;
};

type LoggerLike = Pick<ElectronLogger, "error">;

// Invoke and event handlers share the existing response envelope and error logging.
export function createElectronControllerBinder(
  ipc: IpcMainHandlers,
  logger: LoggerLike,
): ControllerHandlerBinder {
  return ({ handler, event, method }) => {
    ipc[method](event, async (...args: unknown[]) => {
      try {
        return success(await handler(...args));
      } catch (e: unknown) {
        logger.error(`process ipc [${event}] failed: `, e);
        if (e instanceof Error) return error(e.message);
        return error(String(e));
      }
    });
  };
}
