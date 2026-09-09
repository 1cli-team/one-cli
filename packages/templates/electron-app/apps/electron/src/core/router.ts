import { provide } from "@inversifyjs/binding-decorators";
import { ipcMain } from "electron";
import { inject, injectable, multiInject } from "inversify";
import type { AppRouter, Controller } from "../types/core";
import { TYPES } from "../types/symbols";
import ElectronLogger from "../vendor/ElectronLogger";
import { createElectronControllerBinder } from "./electronBinder";
import { registerControllerHandlers } from "./registerControllerHandlers";

// 收集所有 @provide(TYPES.Controller) 的控制器
// 启动时调用 init() 一次性把它们绑定到 ipcMain
@injectable()
@provide()
export default class Router implements AppRouter {
  constructor(
    @multiInject(TYPES.Controller)
    private readonly controllers: Controller[],
    @inject(ElectronLogger)
    private readonly logger: ElectronLogger,
  ) {}

  init(): void {
    const binder = createElectronControllerBinder(ipcMain, this.logger);
    registerControllerHandlers(this.controllers, binder);
  }
}
