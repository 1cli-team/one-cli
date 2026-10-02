import { app, BrowserWindow } from "electron";
import type { ProtocolService } from "./core/protocol";
import type { AppRouter } from "./core/router";
import type { ElectronDevtools } from "./vendor/ElectronDevtools";
import type { ElectronUpdater } from "./vendor/ElectronUpdater";
import type { ElectronLogger } from "./vendor/ElectronLogger";
import type { MainWindow } from "./windows/main.window";

interface AppDependencies {
  mainWindow: MainWindow;
  protocol: ProtocolService;
  router: AppRouter;
  updater: ElectronUpdater;
  devTools: ElectronDevtools;
  logger: ElectronLogger;
}

// Startup order stays explicit and independent of the DI container.
export function createElectronApp({
  mainWindow,
  protocol,
  router,
  updater,
  devTools,
  logger,
}: AppDependencies) {
  const init = async (): Promise<void> => {
    protocol.create();
    router.init();
    await devTools.init();
    await updater.init();

    app.on("activate", () => {
      if (BrowserWindow.getAllWindows().length === 0) mainWindow.init();
    });

    mainWindow.init();
    logger.info("app ready");
  };

  const secondInstance = (): void => {
    mainWindow.init();
  };

  return { init, secondInstance };
}

export type ElectronApp = ReturnType<typeof createElectronApp>;
