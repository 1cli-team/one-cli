import isDev from "electron-is-dev";
import { createWindow } from "../core/window";
import { DEV_RENDERER_URL } from "../constants";
import { defaultScheme, preloadPath } from "../utils";
import type { ElectronStore } from "../vendor/ElectronStore";

// Restore and persist bounds while keeping BrowserWindow state inside the factory.
export function createMainWindow({ store }: { store: ElectronStore }) {
  const bounds = store.get("mainBounds");
  const windowService = createWindow({
    options: {
      width: bounds?.width ?? 1100,
      height: bounds?.height ?? 680,
      x: bounds?.x,
      y: bounds?.y,
      show: false,
      autoHideMenuBar: true,
      webPreferences: {
        preload: preloadPath,
        contextIsolation: true,
        nodeIntegration: false,
      },
    },
    url: isDev ? DEV_RENDERER_URL : `${defaultScheme}://index.html/`,
    onClose: (window) => store.set("mainBounds", window.getBounds()),
  });

  const init = (): void => {
    if (windowService.window) {
      windowService.window.show();
      return;
    }
    windowService.create();
  };

  return {
    get window() {
      return windowService.window;
    },
    init,
    send: windowService.send,
  };
}

export type MainWindow = ReturnType<typeof createMainWindow>;
