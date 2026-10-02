import {
  type IpcMainInvokeEvent,
  Menu,
  type MenuItem,
  type MenuItemConstructorOptions,
} from "electron";
import type { Controller } from "../types/core";
import { IPC, type ContextMenuItem } from "../types/events";

const show = async (
  _event: IpcMainInvokeEvent,
  items: ContextMenuItem[],
): Promise<string | null> =>
  new Promise((resolve) => {
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

export function createContextMenuController(): Controller {
  return { handle: { [IPC.contextMenu.show]: show } };
}
