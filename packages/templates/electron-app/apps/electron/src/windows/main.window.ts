import { provide } from "@inversifyjs/binding-decorators";
import isDev from "electron-is-dev";
import { inject, injectable } from "inversify";
import Window from "../core/window";
import { DEV_RENDERER_URL } from "../constants";
import { defaultScheme, preloadPath } from "../utils";
import ElectronStore from "../vendor/ElectronStore";

// 主窗口
// 生产 URL 走自定义协议，开发 URL 走 Vite 本地服务
// 关闭前把大小/位置存到 ElectronStore，下次启动恢复
@injectable()
@provide()
export default class MainWindow extends Window {
  constructor(
    @inject(ElectronStore)
    private readonly store: ElectronStore,
  ) {
    const bounds = store.get("mainBounds");
    super({
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
    });

    this.url = isDev ? DEV_RENDERER_URL : `${defaultScheme}://index.html/`;
  }

  init(): void {
    if (this.window) {
      this.window.show();
      return;
    }
    this.window = this.create();
    this.window.on("close", () => {
      if (!this.window) return;
      this.store.set("mainBounds", this.window.getBounds());
    });
  }
}
