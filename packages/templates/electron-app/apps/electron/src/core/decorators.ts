import "reflect-metadata";
import { IPC_EVENT_METADATA, IPC_METHOD_METADATA } from "../types/events";

// @handle(channel) —— 把控制器方法标记为 ipcMain.handle 的 handler
// 元数据写在 prototype 上，启动时由 registerControllerHandlers 统一扫描
export const handle = (route: string) => {
  return (target: any, propertyName: string): void => {
    Reflect.defineMetadata(IPC_METHOD_METADATA, "handle", target, propertyName);
    Reflect.defineMetadata(IPC_EVENT_METADATA, route, target, propertyName);
  };
};

// @on(channel) —— ipcMain.on 版本，用于无返回值的"推送"类事件
export const on = (route: string) => {
  return (target: any, propertyName: string): void => {
    Reflect.defineMetadata(IPC_METHOD_METADATA, "on", target, propertyName);
    Reflect.defineMetadata(IPC_EVENT_METADATA, route, target, propertyName);
  };
};
