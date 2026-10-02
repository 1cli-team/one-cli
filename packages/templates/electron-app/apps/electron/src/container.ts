import { asFunction, createContainer, InjectionMode } from "awilix";
import { createElectronApp, type ElectronApp } from "./app";
import {
  createContextMenuController,
  createDialogController,
  createShellController,
} from "./controller";
import { createProtocolService, type ProtocolService } from "./core/protocol";
import { createRouter, type AppRouter } from "./core/router";
import type { Controller } from "./types/core";
import {
  createElectronDevtools,
  type ElectronDevtools,
} from "./vendor/ElectronDevtools";
import {
  createElectronLogger,
  type ElectronLogger,
} from "./vendor/ElectronLogger";
import {
  createElectronStore,
  type ElectronStore,
} from "./vendor/ElectronStore";
import {
  createElectronUpdater,
  type ElectronUpdater,
} from "./vendor/ElectronUpdater";
import { createMainWindow, type MainWindow } from "./windows/main.window";

export interface MainServices {
  logger: ElectronLogger;
  store: ElectronStore;
  mainWindow: MainWindow;
  protocol: ProtocolService;
  router: AppRouter;
  updater: ElectronUpdater;
  devTools: ElectronDevtools;
  dialogController: Controller;
  shellController: Controller;
  contextMenuController: Controller;
  controllers: Controller[];
  application: ElectronApp;
}

// Keep container access in the composition root; services receive plain dependencies.
export function createMainContainer() {
  const container = createContainer<MainServices>({
    injectionMode: InjectionMode.PROXY,
    strict: true,
  });

  container.register({
    logger: asFunction(createElectronLogger).singleton(),
    store: asFunction(createElectronStore).singleton(),
    mainWindow: asFunction(createMainWindow).singleton(),
    protocol: asFunction(createProtocolService).singleton(),
    router: asFunction(createRouter).singleton(),
    updater: asFunction(createElectronUpdater).singleton(),
    devTools: asFunction(createElectronDevtools).singleton(),
    dialogController: asFunction(createDialogController).singleton(),
    shellController: asFunction(createShellController).singleton(),
    contextMenuController: asFunction(createContextMenuController).singleton(),
    controllers: asFunction(
      ({
        dialogController,
        shellController,
        contextMenuController,
      }: Pick<
        MainServices,
        "dialogController" | "shellController" | "contextMenuController"
      >) => [dialogController, shellController, contextMenuController],
    ).singleton(),
    application: asFunction(createElectronApp).singleton(),
  });

  return container;
}
