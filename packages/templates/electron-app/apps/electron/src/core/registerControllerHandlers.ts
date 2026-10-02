import type { Controller, IpcListener } from "../types/core";

export interface ControllerHandlerRegistration {
  handler: IpcListener;
  event: string;
  method: "handle" | "on";
}

export type ControllerHandlerBinder = (
  registration: ControllerHandlerRegistration,
) => void;

// Controllers expose explicit handler maps; no decorators or prototype scanning.
export function registerControllerHandlers(
  controllers: Controller[],
  binder: ControllerHandlerBinder,
): void {
  for (const controller of controllers) {
    for (const [event, handler] of Object.entries(controller.handle)) {
      if (handler)
        binder({ event, method: "handle", handler: handler as IpcListener });
    }
    for (const [event, handler] of Object.entries(controller.on ?? {})) {
      binder({ event, method: "on", handler: handler as IpcListener });
    }
  }
}
