import fs from "node:fs/promises";
import { provide } from "@inversifyjs/binding-decorators";
import { dialog, type IpcMainInvokeEvent } from "electron";
import { inject, injectable } from "inversify";
import { handle } from "../core/decorators";
import type { Controller } from "../types/core";
import {
  IPC,
  type DialogOpenOptions,
  type DialogSaveOptions,
} from "../types/events";
import { TYPES } from "../types/symbols";
import MainWindow from "../windows/main.window";

// 通用文件选择 / 保存对话框
// 约定以主窗口为 parent，避免多个独立对话框淹没用户
@injectable()
@provide(TYPES.Controller)
export default class DialogController implements Controller {
  constructor(
    @inject(MainWindow)
    private readonly mainWindow: MainWindow,
  ) {}

  @handle(IPC.dialog.open)
  async open(
    _e: IpcMainInvokeEvent,
    options: DialogOpenOptions,
  ): Promise<string[]> {
    const window = this.mainWindow.window;
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
      return Promise.all(result.filePaths.map((p) => fs.readFile(p, "utf-8")));
    }
    return result.filePaths;
  }

  @handle(IPC.dialog.save)
  async save(
    _e: IpcMainInvokeEvent,
    options: DialogSaveOptions,
  ): Promise<string> {
    const window = this.mainWindow.window;
    if (!window) throw new Error("main window not ready");

    const result = await dialog.showSaveDialog(window, {
      defaultPath: options.defaultPath,
      filters: options.filters,
    });
    if (result.canceled || !result.filePath) return "";

    await fs.writeFile(result.filePath, options.content, "utf-8");
    return result.filePath;
  }
}
