import { provide } from "@inversifyjs/binding-decorators";
import install, {
  REACT_DEVELOPER_TOOLS,
  REDUX_DEVTOOLS,
} from "electron-devtools-installer";
import isDev from "electron-is-dev";
import { inject, injectable } from "inversify";
import ElectronLogger from "./ElectronLogger";

// 开发时可选安装 React/Redux DevTools
// 走 LOAD_DEVTOOLS 环境变量开关，避免每次启动都去下载
@injectable()
@provide()
export default class ElectronDevtools {
  constructor(
    @inject(ElectronLogger)
    private readonly logger: ElectronLogger,
  ) {}

  async init(): Promise<void> {
    if (!isDev) return;
    if (!process.env.LOAD_DEVTOOLS) return;

    try {
      this.logger.debug("Loading devtools");
      await install([REDUX_DEVTOOLS, REACT_DEVELOPER_TOOLS]);
      this.logger.debug("Devtools loaded");
    } catch (err: unknown) {
      this.logger.error("Failed to load devtools", err);
    }
  }
}
