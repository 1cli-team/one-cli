import isDev from "electron-is-dev";
import pkg from "electron-updater";
import type { ElectronLogger } from "./ElectronLogger";
import type { MainWindow } from "../windows/main.window";
import { EVENT } from "../types/events";

const { autoUpdater } = pkg;

export function createElectronUpdater({
  logger,
  mainWindow,
}: {
  logger: ElectronLogger;
  mainWindow: MainWindow;
}) {
  const check = async (): Promise<void> => {
    try {
      await autoUpdater.checkForUpdates();
    } catch (error: unknown) {
      logger.error("check for updates failed", error);
    }
  };

  const init = async (
    options: { autoUpgrade?: boolean; allowBeta?: boolean } = {},
  ): Promise<void> => {
    autoUpdater.disableWebInstaller = true;
    autoUpdater.logger = logger.logger;
    autoUpdater.allowPrerelease = options.allowBeta ?? false;
    if (isDev) autoUpdater.forceDevUpdateConfig = true;
    autoUpdater.autoDownload = options.autoUpgrade ?? false;

    autoUpdater.on("checking-for-update", () =>
      mainWindow.send(EVENT.update.checking),
    );
    autoUpdater.on("update-available", () =>
      mainWindow.send(EVENT.update.available),
    );
    autoUpdater.on("update-not-available", () =>
      mainWindow.send(EVENT.update.notAvailable),
    );
    autoUpdater.on("download-progress", (progress) =>
      mainWindow.send(EVENT.update.downloadProgress, progress),
    );
    autoUpdater.on("update-downloaded", () =>
      mainWindow.send(EVENT.update.downloaded),
    );

    setTimeout(() => void check(), 60_000);
  };

  return {
    init,
    check,
    startDownload: (): Promise<string[]> => autoUpdater.downloadUpdate(),
    install: (): void => autoUpdater.quitAndInstall(),
    setAllowBeta: (allow: boolean): void => {
      autoUpdater.allowPrerelease = allow;
    },
  };
}

export type ElectronUpdater = ReturnType<typeof createElectronUpdater>;
