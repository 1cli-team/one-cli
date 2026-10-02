import { contextBridge, ipcRenderer, type IpcRendererEvent } from "electron";
import type { ElectronAPI } from "./api";
import type { IpcResponse } from "./channels";

// 解包响应信封：code === 0 返回 data，否则抛 Error(message)
// 上层调用方写普通的 try/catch 就能处理失败
function unwrap<T>(res: IpcResponse<T> | T): T {
  if (
    res &&
    typeof res === "object" &&
    "code" in (res as object) &&
    "message" in (res as object)
  ) {
    const envelope = res as IpcResponse<T>;
    if (envelope.code === 0) return envelope.data as T;
    throw new Error(envelope.message || "ipc error");
  }
  return res as T;
}

const api: ElectronAPI = {
  async invoke(channel, ...args) {
    const res = await ipcRenderer.invoke(channel as string, ...args);
    return unwrap(res);
  },

  on(channel, listener) {
    const wrapped = (_e: IpcRendererEvent, ...args: unknown[]) =>
      listener(...args);
    ipcRenderer.on(channel as string, wrapped);
    return () => ipcRenderer.removeListener(channel as string, wrapped);
  },

  once(channel, listener) {
    const wrapped = (_e: IpcRendererEvent, ...args: unknown[]) =>
      listener(...args);
    ipcRenderer.once(channel as string, wrapped);
    return () => ipcRenderer.removeListener(channel as string, wrapped);
  },
};

contextBridge.exposeInMainWorld("electron", api);

export type { ElectronAPI } from "./api";
export type {
  IpcResponse,
  InvokeChannel,
  InvokeMap,
  EventChannel,
  DialogOpenOptions,
  DialogSaveOptions,
  ContextMenuItem,
} from "./channels";
export { IPC, EVENT } from "./channels";
