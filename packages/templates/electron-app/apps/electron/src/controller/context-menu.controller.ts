import { provide } from "@inversifyjs/binding-decorators";
import {
  type IpcMainInvokeEvent,
  Menu,
  type MenuItem,
  type MenuItemConstructorOptions,
} from "electron";
import { injectable } from "inversify";
import { handle } from "../core/decorators";
import type { Controller } from "../types/core";
import { IPC, type ContextMenuItem } from "../types/events";
import { TYPES } from "../types/symbols";

// 让渲染端在原生 context menu 里弹出选项
// 返回用户点击的 key（取消返回 null）
@injectable()
@provide(TYPES.Controller)
export default class ContextMenuController implements Controller {
  @handle(IPC.contextMenu.show)
  async show(
    _e: IpcMainInvokeEvent,
    items: ContextMenuItem[],
  ): Promise<string | null> {
    return new Promise((resolve) => {
      let resolved = false;
      const template: Array<MenuItemConstructorOptions | MenuItem> = items.map(
        (item) => {
          if (item.type === "separator") return { type: "separator" as const };
          return {
            label: item.label,
            click: () => {
              resolved = true;
              resolve(item.key ?? null);
            },
          };
        },
      );
      const menu = Menu.buildFromTemplate(template);
      menu.popup({
        callback: () => {
          if (!resolved) resolve(null);
        },
      });
    });
  }
}
