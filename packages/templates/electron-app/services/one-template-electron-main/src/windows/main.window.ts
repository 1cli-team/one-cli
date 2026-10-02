import { BrowserWindow } from "electron";
import { pathToFileURL } from "node:url";
import { DEV_RENDERER_URL } from "../constants";
import { preloadPath, rendererPath } from "../utils";

export function createMainWindow() {
  let window: BrowserWindow | null = null;
  const development = process.env.NODE_ENV === "development";
  const trustedURL = development
    ? DEV_RENDERER_URL
    : pathToFileURL(rendererPath).href;
  const isTrustedURL = (url: string) => url.split("#")[0] === trustedURL;
  const init = async (): Promise<void> => {
    if (window) {
      if (window.isMinimized()) window.restore();
      window.show();
      window.focus();
      return;
    }
    const current = new BrowserWindow({
      width: 1100,
      height: 680,
      show: false,
      autoHideMenuBar: true,
      webPreferences: {
        preload: preloadPath,
        contextIsolation: true,
        nodeIntegration: false,
        sandbox: true,
      },
    });
    window = current;
    current.webContents.setWindowOpenHandler(() => ({ action: "deny" }));
    current.webContents.on("will-navigate", (event) => event.preventDefault());
    current.once("closed", () => {
      if (window === current) window = null;
    });
    current.once("ready-to-show", () => current.show());
    try {
      if (development) await current.loadURL(DEV_RENDERER_URL);
      else await current.loadFile(rendererPath);
    } catch (error) {
      if (!current.isDestroyed()) current.destroy();
      if (window === current) window = null;
      throw error;
    }
  };
  return {
    get window() {
      return window;
    },
    init,
    isTrustedURL,
  };
}
export type MainWindow = ReturnType<typeof createMainWindow>;
