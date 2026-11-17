import { app, dialog } from "electron";
import { asFunction, asValue, createContainer, InjectionMode } from "awilix";
import { readFile } from "node:fs/promises";
import { join, resolve } from "node:path";
import { createBackend, type Manifest } from "./backend.ts";
import { createMainWindow, registerAppScheme, APP_ORIGIN, DEV_ORIGIN } from "./window.ts";
import { createLifecycle } from "./lifecycle.ts";
import { createDesktopIPC } from "./ipc.ts";

registerAppScheme();
if (process.platform === "darwin") app.setAboutPanelOptions({ applicationName: "One Desktop" });
app.setName("One Desktop");
if (process.platform === "linux") app.commandLine.appendSwitch("class", "one_desktop");
app.setAppUserModelId("team.1cli.desktop");
if (process.env.ONE_DESKTOP_USER_DATA) {
  app.setPath("userData", resolve(process.env.ONE_DESKTOP_USER_DATA));
}

async function start() {
  if (!app.requestSingleInstanceLock()) {
    app.quit();
  } else {
    await app.whenReady();
    const development = !app.isPackaged && process.env.ONE_DESKTOP_DEV === "1";
    const root = app.getAppPath();
    const buildDirectory = join(root, "build");
    const assetsDirectory = join(root, "assets");
    const resourcesDirectory = app.isPackaged ? process.resourcesPath : join(root, "resources");
    const manifest: Manifest = JSON.parse(
      await readFile(join(resourcesDirectory, "bundle-manifest.json"), "utf8"),
    );
    const container = createContainer({ injectionMode: InjectionMode.PROXY, strict: true });
    container.register({
      development: asValue(development),
      buildDirectory: asValue(buildDirectory),
      assetsDirectory: asValue(assetsDirectory),
      backend: asFunction(() =>
        createBackend({
          executable: join(
            resourcesDirectory,
            "bin",
            process.platform === "win32" ? "one-api.exe" : "one-api",
          ),
          manifest,
          origin: development ? DEV_ORIGIN : APP_ORIGIN,
        }),
      ).singleton(),
      mainWindow: asFunction(createMainWindow).singleton(),
      lifecycle: asFunction(createLifecycle).singleton(),
      ipc: asFunction(createDesktopIPC).singleton(),
    });
    const backend = container.resolve<ReturnType<typeof createBackend>>("backend");
    const mainWindow = container.resolve<ReturnType<typeof createMainWindow>>("mainWindow");
    const lifecycle = container.resolve<ReturnType<typeof createLifecycle>>("lifecycle");
    const ipc = container.resolve<ReturnType<typeof createDesktopIPC>>("ipc");
    try {
      lifecycle.init();
      const unsubscribe = ipc.init();
      app.once("will-quit", () => {
        unsubscribe();
        ipc.dispose();
        void container.dispose();
      });
      await Promise.all([backend.start(), mainWindow.init()]);
    } catch {
      dialog.showErrorBox(
        "One Desktop",
        "One Desktop could not start. Reinstall the application.\nOne Desktop 无法启动，请重新安装应用。",
      );
      await backend.stop();
      app.exit(1);
    }
    for (const signal of ["SIGINT", "SIGTERM"] as const) {
      process.once(signal, () => {
        void backend.stop().finally(() => app.exit(0));
      });
    }
  }
}
void start().catch(() => {
  dialog.showErrorBox(
    "One Desktop",
    "One Desktop could not start. Reinstall the application.\nOne Desktop 无法启动，请重新安装应用。",
  );
  app.exit(1);
});
