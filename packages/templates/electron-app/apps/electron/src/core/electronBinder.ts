import type { ControllerHandlerBinder } from "./registerControllerHandlers";
import { error, success } from "../utils/ipcResponse";
import type ElectronLogger from "../vendor/ElectronLogger";

export type IpcMainHandlers = {
  handle: (channel: string, listener: (...args: unknown[]) => unknown) => void;
  on: (channel: string, listener: (...args: unknown[]) => unknown) => void;
};

type LoggerLike = Pick<ElectronLogger, "error">;

function isPromiseLike(value: unknown): value is PromiseLike<unknown> {
  return typeof (value as PromiseLike<unknown>)?.then === "function";
}

// 把扫描到的 handler 真正绑到 ipcMain 上
// 所有处理器统一加 try/catch + 走 success/error 信封
export function createElectronControllerBinder(
  ipc: IpcMainHandlers,
  logger: LoggerLike,
): ControllerHandlerBinder {
  return ({ controller, handler, event, method }) => {
    if (method !== "on" && method !== "handle") return;

    ipc[method](event, async (...args: unknown[]) => {
      try {
        let result: unknown = handler.call(controller, ...args);
        if (isPromiseLike(result)) {
          result = await result;
        }
        return success(result);
      } catch (e: unknown) {
        logger.error(`process ipc [${event}] failed: `, e);
        if (e instanceof Error) return error(e.message);
        return error(String(e));
      }
    });
  };
}
