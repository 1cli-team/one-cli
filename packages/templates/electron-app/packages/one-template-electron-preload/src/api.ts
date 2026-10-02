import type { InvokeChannel, InvokeMap, EventChannel } from "./channels";

// 暴露到 renderer window.electron 上的 API 契约
export interface ElectronAPI {
  // invoke：有返回值，preload 内部已解包信封，失败会抛错
  invoke<K extends InvokeChannel>(
    channel: K,
    ...args: InvokeMap[K]["args"]
  ): Promise<InvokeMap[K]["return"]>;

  // on：订阅事件，返回 unsubscribe 函数
  on<K extends EventChannel | string>(
    channel: K,
    listener: (...args: unknown[]) => void,
  ): () => void;

  // once：订阅一次，返回 unsubscribe（提前取消也安全）
  once<K extends EventChannel | string>(
    channel: K,
    listener: (...args: unknown[]) => void,
  ): () => void;
}

declare global {
  interface Window {
    electron: ElectronAPI;
  }
}
