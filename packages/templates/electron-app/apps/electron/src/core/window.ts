import { BrowserWindow, type BrowserWindowConstructorOptions } from "electron";
import isDev from "electron-is-dev";

interface WindowOptions {
  options: BrowserWindowConstructorOptions;
  url: string;
  onClose?: (window: BrowserWindow) => void;
}

// Each factory call owns its window reference; callbacks share it through a closure.
export function createWindow({ options, url, onClose }: WindowOptions) {
  let window: BrowserWindow | null = null;

  const create = (): BrowserWindow => {
    if (!url) throw new Error("url is required");

    const current = new BrowserWindow(options);
    window = current;
    void current.loadURL(url);

    current.once("ready-to-show", () => {
      current.show();
      if (isDev) current.webContents.openDevTools();
    });
    current.on("close", () => onClose?.(current));
    current.once("closed", () => {
      if (window === current) window = null;
    });

    return current;
  };

  const send = (channel: string, ...args: unknown[]): void => {
    window?.webContents.send(channel, ...args);
  };

  return {
    get window() {
      return window;
    },
    create,
    send,
  };
}
