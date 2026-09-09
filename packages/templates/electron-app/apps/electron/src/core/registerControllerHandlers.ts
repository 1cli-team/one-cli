import "reflect-metadata";
import type { Controller } from "../types/core";
import { IPC_EVENT_METADATA, IPC_METHOD_METADATA } from "../types/events";

export interface ControllerHandlerRegistration {
  controller: Controller;
  propertyKey: string | symbol;
  handler: (...args: unknown[]) => unknown;
  event: string;
  method: string;
}

export type ControllerHandlerBinder = (
  registration: ControllerHandlerRegistration,
) => void;

// 扫描控制器原型上带 @handle/@on 元数据的方法，交给 binder 处理
// 不直接操作 ipcMain —— binder 抽象了"如何绑定"，便于单测时传假实现
export function registerControllerHandlers(
  controllers: Controller[],
  binder: ControllerHandlerBinder,
): void {
  for (const controller of controllers) {
    if (!controller) continue;
    const prototype = Object.getPrototypeOf(controller);
    if (!prototype) continue;

    for (const propertyKey of Reflect.ownKeys(prototype)) {
      if (propertyKey === "constructor") continue;
      const handler = Reflect.get(controller as object, propertyKey);
      if (typeof handler !== "function") continue;

      const event = Reflect.getMetadata(
        IPC_EVENT_METADATA,
        controller,
        propertyKey as string,
      );
      const method = Reflect.getMetadata(
        IPC_METHOD_METADATA,
        controller,
        propertyKey as string,
      );

      if (typeof event !== "string" || typeof method !== "string") continue;

      binder({
        controller,
        propertyKey,
        handler: handler as (...args: unknown[]) => unknown,
        event,
        method,
      });
    }
  }
}
