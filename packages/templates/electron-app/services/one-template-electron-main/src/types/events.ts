// IPC 通道常量从 preload 包 re-export —— 主进程和 renderer 共用同一份源
export { IPC, EVENT } from "@one-template-electron/preload/channels";
export type {
  IpcResponse,
  InvokeMap,
  InvokeChannel,
  EventChannel,
  DialogOpenOptions,
  DialogSaveOptions,
  ContextMenuItem,
} from "@one-template-electron/preload/channels";
