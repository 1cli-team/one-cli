import { asFunction, createContainer, InjectionMode } from "awilix";
import { createElectronApp, type ElectronApp } from "./app";
import { createAppIPC, type AppIPC } from "./ipc";
import {
  createElectronLogger,
  type ElectronLogger,
} from "./vendor/ElectronLogger";
import { createMainWindow, type MainWindow } from "./windows/main.window";

interface MainServices {
  logger: ElectronLogger;
  mainWindow: MainWindow;
  ipc: AppIPC;
  application: ElectronApp;
}
// Only the composition root accesses the container. Factories receive plain dependencies.
export function createMainContainer() {
  const container = createContainer<MainServices>({
    injectionMode: InjectionMode.PROXY,
    strict: true,
  });
  container.register({
    logger: asFunction(createElectronLogger).singleton(),
    mainWindow: asFunction(createMainWindow).singleton(),
    ipc: asFunction(createAppIPC).singleton(),
    application: asFunction(createElectronApp)
      .singleton()
      .disposer((application) => application.dispose()),
  });
  return container;
}
