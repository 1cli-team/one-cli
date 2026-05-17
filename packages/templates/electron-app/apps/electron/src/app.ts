import { app, BrowserWindow } from "electron";
import { provide } from "@inversifyjs/binding-decorators";
import { inject, injectable } from "inversify";
import ProtocolService from "./core/protocol";
import Router from "./core/router";
import ElectronDevtools from "./vendor/ElectronDevtools";
import ElectronUpdater from "./vendor/ElectronUpdater";
import ElectronLogger from "./vendor/ElectronLogger";
import MainWindow from "./windows/main.window";
import "./controller";

// 编排器：定义启动顺序
// 1. 协议注册（whenReady 后尽早）
// 2. IPC 路由（绑定所有控制器）
// 3. Vendor init（日志、更新器、DevTools）
// 4. 平台事件（activate / second-instance / quit）
// 5. 业务窗口创建
@injectable()
@provide()
export default class ElectronApp {
  constructor(
    @inject(MainWindow)       private readonly mainWindow: MainWindow,
    @inject(ProtocolService)  private readonly protocol: ProtocolService,
    @inject(Router)           private readonly router: Router,
    @inject(ElectronUpdater)  private readonly updater: ElectronUpdater,
    @inject(ElectronDevtools) private readonly devTools: ElectronDevtools,
    @inject(ElectronLogger)   private readonly logger: ElectronLogger,
  ) {}

  async init(): Promise<void> {
    this.protocol.create();
    this.router.init();

    await this.devTools.init();
    await this.updater.init();

    app.on("activate", () => {
      if (BrowserWindow.getAllWindows().length === 0) this.mainWindow.init();
    });

    this.mainWindow.init();

    this.logger.info("app ready");
  }

  // 第二实例启动时把焦点还给已有窗口
  secondInstance = (): void => {
    this.mainWindow.init();
  };
}
