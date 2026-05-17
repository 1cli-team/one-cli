import "reflect-metadata";
import { buildProviderModule } from "@inversifyjs/binding-decorators";
import { app, protocol } from "electron";
import { Container } from "inversify";
import ElectronApp from "./app";
import { defaultScheme, noop } from "./utils";

// 1. 容器：所有类默认单例
const container = new Container({ defaultScope: "Singleton" });

// 2. 单实例锁：重复启动时把焦点交给已有实例
const gotTheLock = app.requestSingleInstanceLock();

const start = async (): Promise<void> => {
  if (!gotTheLock) {
    app.quit();
    return;
  }

  // 3. 自定义协议注册为特权 scheme（支持 fetch / cookies / service worker）
  //    必须在 app ready 之前调用
  protocol.registerSchemesAsPrivileged([
    { scheme: defaultScheme, privileges: { secure: true, standard: true } },
  ]);

  await app.whenReady();

  // 4. 加载所有 @provide 装饰的类到容器
  await container.load(buildProviderModule());

  // 5. 取编排器并启动
  const application = container.get(ElectronApp);
  await application.init();

  // 6. 平台惯例：macOS 关闭所有窗口不退出
  app.on("window-all-closed", noop);
  app.on("second-instance", application.secondInstance);
};

void start();
