import { app } from "electron";
import { createMainContainer } from "./container";

async function start() {
  if (!app.requestSingleInstanceLock()) {
    app.quit();
    return;
  }
  await app.whenReady();
  const container = createMainContainer();
  const application = container.resolve("application");
  app.once("will-quit", () => {
    application.dispose();
    void container.dispose();
  });
  try {
    await application.init();
  } catch (error) {
    container.resolve("logger").error(error);
    application.dispose();
    app.exit(1);
  }
}
void start();
