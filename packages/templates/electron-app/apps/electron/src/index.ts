import { app, protocol } from "electron";
import { createMainContainer } from "./container";
import { defaultScheme, noop } from "./utils";

const gotTheLock = app.requestSingleInstanceLock();

const start = async (): Promise<void> => {
  if (!gotTheLock) {
    app.quit();
    return;
  }

  // Privileged schemes must be registered before Electron is ready.
  protocol.registerSchemesAsPrivileged([
    { scheme: defaultScheme, privileges: { secure: true, standard: true } },
  ]);

  await app.whenReady();

  const container = createMainContainer();
  const application = container.resolve("application");
  await application.init();

  app.on("window-all-closed", noop);
  app.on("second-instance", application.secondInstance);
};

void start();
