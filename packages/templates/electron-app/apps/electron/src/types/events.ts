// 元数据 key：装饰器写入，注册时读取。用 Symbol 避免和用户字段冲突
export const IPC_EVENT_METADATA = Symbol.for("app:ipc-event");
export const IPC_METHOD_METADATA = Symbol.for("app:ipc-method");

// IPC 通道常量从 preload 包 re-export —— 主进程和 renderer 共用同一份源
export { IPC, EVENT } from "@app/preload/channels";
export type {
  IpcResponse,
  InvokeMap,
  InvokeChannel,
  EventChannel,
  DialogOpenOptions,
  DialogSaveOptions,
  ContextMenuItem,
} from "@app/preload/channels";
