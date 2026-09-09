import { provide } from "@inversifyjs/binding-decorators";
import Store from "electron-store";
import { injectable } from "inversify";
import { workspace } from "../utils";

// 持久化小量结构化数据（窗口位置、用户偏好等）
// 大数据别塞这里，走单独的存储层
interface AppPersistentStore {
  mainBounds?: Electron.Rectangle;
  // 按需扩展
}

@injectable()
@provide()
export default class ElectronStore extends Store<AppPersistentStore> {
  constructor() {
    super({
      name: "window-state",
      cwd: workspace,
      defaults: {},
    });
  }
}
