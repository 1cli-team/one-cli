import { ipcMain, dialog, shell } from "electron";
import type { MainWindow } from "./window.ts";
import type { Backend } from "./backend.ts";
import type { createLifecycle } from "./lifecycle.ts";

export function createDesktopIPC({
  mainWindow,
  backend,
  lifecycle,
}: {
  mainWindow: MainWindow;
  backend: Backend;
  lifecycle: ReturnType<typeof createLifecycle>;
}) {
  const channels: string[] = [];
  function handle(channel: string, count: number, work: (...args: unknown[]) => unknown) {
    channels.push(channel);
    ipcMain.handle(channel, (event, ...args: unknown[]) => {
      mainWindow.assertTrusted(event);
      if (args.length !== count) throw new Error("invalid_arguments");
      return work(...args);
    });
  }
  function init() {
    handle("desktop:legacy-preferences", 0, () => mainWindow.getLegacyPreferences());
    handle("desktop:connection", 0, () => backend.connection());
    handle("desktop:retry", 0, () => backend.retry());
    handle("desktop:health", 1, (generation) => {
      if (typeof generation !== "string") throw new Error("invalid_arguments");
      return backend.confirmHealth(generation);
    });
    handle("desktop:locale", 1, (locale) => {
      if (typeof locale !== "string") throw new Error("invalid_locale");
      lifecycle.setLocale(locale);
    });
    handle("desktop:choose-workspace", 0, async () => {
      const result = await dialog.showOpenDialog(mainWindow.window!, {
        properties: ["openDirectory"],
      });
      return result.canceled ? null : (result.filePaths[0] ?? null);
    });
    handle("desktop:open-external", 1, (value) => {
      if (typeof value !== "string" || value.length > 16384) throw new Error("invalid_url");
      let url: URL;
      try {
        url = new URL(value);
      } catch {
        throw new Error("invalid_url");
      }
      if (
        !["https:", "http:"].includes(url.protocol) ||
        !url.hostname ||
        url.username ||
        url.password
      ) {
        throw new Error("invalid_url");
      }
      return shell.openExternal(url.href);
    });
    // The released Tauri app does not enable its updater. Keep that exact UI
    // contract until signed Electron feeds and recovery are explicitly enabled.
    handle("desktop:check-update", 0, () => ({ enabled: false, reason: "update_unconfigured" }));
    handle("desktop:install-update", 0, () => {
      throw new Error("update_recovery_required");
    });
    return backend.onState(() => {
      const window = mainWindow.window;
      if (window && !window.isDestroyed()) window.webContents.send("desktop:state");
    });
  }
  return {
    init,
    dispose() {
      channels.forEach((channel) => ipcMain.removeHandler(channel));
    },
  };
}
