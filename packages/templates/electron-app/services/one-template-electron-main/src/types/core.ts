import type { IpcMainEvent, IpcMainInvokeEvent } from "electron";
import type { InvokeChannel, InvokeMap } from "./events";

export type InvokeHandlers = {
  [Channel in InvokeChannel]?: (
    event: IpcMainInvokeEvent,
    ...args: InvokeMap[Channel]["args"]
  ) => InvokeMap[Channel]["return"] | Promise<InvokeMap[Channel]["return"]>;
};

export type IpcListener = (...args: unknown[]) => unknown;

export interface Controller {
  handle: InvokeHandlers;
  on?: Record<string, (event: IpcMainEvent, ...args: unknown[]) => unknown>;
}
