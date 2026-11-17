import { app, dialog, Menu, Tray, nativeImage } from "electron";
import { join } from "node:path";
import type { MainWindow } from "./window.ts";
import type { Backend } from "./backend.ts";
import english from "../../../apps/desktop-renderer/src/locales/en-US.json" with { type: "json" };
import chinese from "../../../apps/desktop-renderer/src/locales/zh-CN.json" with { type: "json" };

export function createLifecycle({
  mainWindow,
  backend,
  assetsDirectory,
}: {
  mainWindow: MainWindow;
  backend: Backend;
  assetsDirectory: string;
}) {
  let tray: Tray | undefined;
  let closing = false;
  let finished = false;
  let locale = "en-US";
  const messages = () => (locale === "zh-CN" ? chinese : english).desktop;

  async function requestExit() {
    if (closing || finished) return;
    closing = true;
    try {
      const active = await backend.quiesce();
      if (active > 0) {
        const text = messages().quit;
        const result = await dialog.showMessageBox({
          title: "One Desktop",
          type: "question",
          message: text.running,
          buttons: [text.cancel, text.confirm],
          cancelId: 0,
          defaultId: 0,
          noLink: true,
        });
        if (result.response !== 1) {
          backend.resume();
          closing = false;
          return;
        }
      }
      await backend.stop();
      finished = true;
      tray?.destroy();
      mainWindow.permitExit();
      app.quit();
    } catch (error) {
      try {
        backend.resume();
      } catch {
        /* Backend may already be stopped. */
      }
      closing = false;
      const code = (error as Error).message;
      const text = messages();
      await dialog.showMessageBox({
        title: "One Desktop",
        type: "error",
        message: (text.native as Record<string, string>)[code] ?? text.quit.failed,
      });
    }
  }

  function setLocale(value: string) {
    if (value !== "zh-CN" && value !== "en-US") throw new Error("invalid_locale");
    locale = value;
    const text = messages();
    const items: Electron.MenuItemConstructorOptions[] = [
      { label: text.tray.show, click: () => mainWindow.show() },
      { type: "separator" },
      {
        label: text.tray.quit,
        click: () => {
          void requestExit();
        },
      },
    ];
    tray?.setContextMenu(Menu.buildFromTemplate(items));
  }

  function init() {
    const icon =
      process.platform === "darwin"
        ? nativeImage.createFromPath(join(assetsDirectory, "trayTemplate.png"))
        : process.platform === "win32"
          ? nativeImage.createFromPath(join(assetsDirectory, "icon.ico"))
          : nativeImage.createFromPath(join(assetsDirectory, "icon.png")).resize({ width: 32 });
    // Preserve the template's @2x representation and let macOS apply its menu bar colors.
    if (process.platform === "darwin") icon.setTemplateImage(true);
    tray = new Tray(icon);
    tray.setToolTip("One Desktop");
    tray.on("double-click", () => mainWindow.show());
    setLocale(app.getLocale().startsWith("zh") ? "zh-CN" : "en-US");
    app.on("activate", () => mainWindow.show());
    app.on("second-instance", () => mainWindow.show());
    app.on("before-quit", (event) => {
      if (!finished) {
        event.preventDefault();
        void requestExit();
      }
    });
    // A second launch restores hidden windows even without a desktop tray.
    app.on("window-all-closed", () => {
      /* Closing the window keeps the backend alive. */
    });
  }

  return { init, setLocale, requestExit };
}
