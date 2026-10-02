import { ipcMain } from "electron";
import type { Controller } from "../types/core";
import type { ElectronLogger } from "../vendor/ElectronLogger";
import { createElectronControllerBinder } from "./electronBinder";
import { registerControllerHandlers } from "./registerControllerHandlers";

export function createRouter({
  controllers,
  logger,
}: {
  controllers: Controller[];
  logger: ElectronLogger;
}) {
  const init = (): void => {
    const binder = createElectronControllerBinder(ipcMain, logger);
    registerControllerHandlers(controllers, binder);
  };

  return { init };
}

export type AppRouter = ReturnType<typeof createRouter>;
