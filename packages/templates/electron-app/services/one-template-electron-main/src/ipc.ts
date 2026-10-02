import { app, ipcMain } from "electron";
import { IPC, type AppInfo } from "./types/events";
import type { MainWindow } from "./windows/main.window";

export function createAppIPC({ mainWindow }: { mainWindow: MainWindow }) {
  const init = () => {
    ipcMain.handle(IPC.appInfo, (event, ...args: unknown[]): AppInfo => {
      const contents = mainWindow.window?.webContents;
      if (
        !contents ||
        event.sender !== contents ||
        !event.senderFrame ||
        event.senderFrame !== contents.mainFrame ||
        !mainWindow.isTrustedURL(event.senderFrame.url) ||
        args.length !== 0
      ) {
        throw new Error("Unauthorized IPC request");
      }
      return {
        name: app.getName(),
        version: app.getVersion(),
        platform: process.platform,
      };
    });
  };
  const dispose = () => ipcMain.removeHandler(IPC.appInfo);
  return { init, dispose };
}
export type AppIPC = ReturnType<typeof createAppIPC>;
