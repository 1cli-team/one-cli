import { app } from "electron";
import type { AppIPC } from "./ipc";
import type { ElectronLogger } from "./vendor/ElectronLogger";
import type { MainWindow } from "./windows/main.window";

export function createElectronApp({
  mainWindow,
  ipc,
  logger,
}: {
  mainWindow: MainWindow;
  ipc: AppIPC;
  logger: ElectronLogger;
}) {
  const show = () => {
    void mainWindow.init().catch((error) => logger.error(error));
  };
  const closed = () => {
    if (process.platform !== "darwin") app.quit();
  };
  let initialized = false;
  const init = async () => {
    if (initialized) return;
    initialized = true;
    ipc.init();
    app.on("activate", show);
    app.on("second-instance", show);
    app.on("window-all-closed", closed);
    await mainWindow.init();
    logger.info("app ready");
  };
  const dispose = () => {
    if (!initialized) return;
    initialized = false;
    ipc.dispose();
    app.removeListener("activate", show);
    app.removeListener("second-instance", show);
    app.removeListener("window-all-closed", closed);
  };
  return { init, dispose };
}
export type ElectronApp = ReturnType<typeof createElectronApp>;
