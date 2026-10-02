import { type IpcMainInvokeEvent, shell } from "electron";
import type { Controller } from "../types/core";
import { IPC } from "../types/events";

const open = async (
  _event: IpcMainInvokeEvent,
  target: string,
): Promise<void> => {
  if (/^https?:\/\//i.test(target)) {
    await shell.openExternal(target);
  } else {
    await shell.openPath(target);
  }
};

export function createShellController(): Controller {
  return { handle: { [IPC.shell.open]: open } };
}
