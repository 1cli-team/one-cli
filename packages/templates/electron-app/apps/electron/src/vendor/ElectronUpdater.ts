import { provide } from "@inversifyjs/binding-decorators";
import isDev from "electron-is-dev";
import pkg from "electron-updater";
import { inject, injectable } from "inversify";
import ElectronLogger from "./ElectronLogger";
import MainWindow from "../windows/main.window";
import { EVENT } from "../types/events";

const { autoUpdater } = pkg;

// electron-updater 薄包装
// 把更新状态事件转发到渲染端，UI 订阅 update:* 通道展示进度
@injectable()
@provide()
export default class ElectronUpdater {
  constructor(
    @inject(ElectronLogger)
    private readonly logger: ElectronLogger,
    @inject(MainWindow)
    private readonly mainWindow: MainWindow,
  ) {}

  async init(options: { autoUpgrade?: boolean; allowBeta?: boolean } = {}): Promise<void> {
    autoUpdater.disableWebInstaller = true;
    autoUpdater.logger = this.logger.logger;
    autoUpdater.allowPrerelease = options.allowBeta ?? false;
    if (isDev) autoUpdater.forceDevUpdateConfig = true;

    autoUpdater.autoDownload = options.autoUpgrade ?? false;

    autoUpdater.on("checking-for-update", () =>
      this.mainWindow.send(EVENT.update.checking),
    );
    autoUpdater.on("update-available", () =>
      this.mainWindow.send(EVENT.update.available),
    );
    autoUpdater.on("update-not-available", () =>
      this.mainWindow.send(EVENT.update.notAvailable),
    );
    autoUpdater.on("download-progress", (progress) =>
      this.mainWindow.send(EVENT.update.downloadProgress, progress),
    );
    autoUpdater.on("update-downloaded", () =>
      this.mainWindow.send(EVENT.update.downloaded),
    );

    // 启动后延迟 60s 检查，避免抢占冷启动资源
    setTimeout(() => void this.check(), 60_000);
  }

  async check(): Promise<void> {
    try {
      await autoUpdater.checkForUpdates();
    } catch (e) {
      this.logger.error("check for updates failed", e);
    }
  }

  startDownload(): Promise<string[]> {
    return autoUpdater.downloadUpdate();
  }

  install(): void {
    autoUpdater.quitAndInstall();
  }

  setAllowBeta(allow: boolean): void {
    autoUpdater.allowPrerelease = allow;
  }
}
