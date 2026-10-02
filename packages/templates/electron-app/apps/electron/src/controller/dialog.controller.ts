import fs from "node:fs/promises";
import { dialog, type IpcMainInvokeEvent } from "electron";
import type { Controller } from "../types/core";
import {
  IPC,
  type DialogOpenOptions,
  type DialogSaveOptions,
} from "../types/events";
import type { MainWindow } from "../windows/main.window";

export function createDialogController({
  mainWindow,
}: {
  mainWindow: MainWindow;
}): Controller {
  const open = async (
    _event: IpcMainInvokeEvent,
    options: DialogOpenOptions,
  ): Promise<string[]> => {
    const window = mainWindow.window;
    if (!window) throw new Error("main window not ready");

    const properties: Electron.OpenDialogOptions["properties"] =
      options.type === "directory" ? ["openDirectory"] : ["openFile"];
    if (options.multiple) properties.push("multiSelections");

    const result = await dialog.showOpenDialog(window, {
      properties,
      filters: options.filters,
    });
    if (result.canceled) return [];
    if (options.readContent && options.type === "file") {
      return Promise.all(
        result.filePaths.map((filePath) => fs.readFile(filePath, "utf-8")),
      );
    }
    return result.filePaths;
  };

  const save = async (
    _event: IpcMainInvokeEvent,
    options: DialogSaveOptions,
  ): Promise<string> => {
    const window = mainWindow.window;
    if (!window) throw new Error("main window not ready");

    const result = await dialog.showSaveDialog(window, {
      defaultPath: options.defaultPath,
      filters: options.filters,
    });
    if (result.canceled || !result.filePath) return "";
    await fs.writeFile(result.filePath, options.content, "utf-8");
    return result.filePath;
  };

  return { handle: { [IPC.dialog.open]: open, [IPC.dialog.save]: save } };
}
