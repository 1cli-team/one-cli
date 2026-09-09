import { provide } from "@inversifyjs/binding-decorators";
import { type IpcMainInvokeEvent, shell } from "electron";
import { injectable } from "inversify";
import { handle } from "../core/decorators";
import type { Controller } from "../types/core";
import { IPC } from "../types/events";
import { TYPES } from "../types/symbols";

// 跨 URL / 文件路径的"打开"统一入口
@injectable()
@provide(TYPES.Controller)
export default class ShellController implements Controller {
  @handle(IPC.shell.open)
  async open(_e: IpcMainInvokeEvent, target: string): Promise<void> {
    if (/^https?:\/\//i.test(target)) {
      await shell.openExternal(target);
    } else {
      await shell.openPath(target);
    }
  }
}
