// IPC 通道：invoke（request/response）类型
// 前后端共用同一份常量，拼写错误在编译期就能发现
export const IPC = {
  dialog: {
    open: "dialog.open",
    save: "dialog.save",
  },
  shell: {
    open: "shell.open",
  },
  contextMenu: {
    show: "contextMenu.show",
  },
  update: {
    check: "update.check",
    startDownload: "update.startDownload",
    install: "update.install",
  },
} as const;

// 主进程主动推送到 renderer 的事件通道（无返回值）
export const EVENT = {
  update: {
    checking: "update:checking",
    available: "update:available",
    notAvailable: "update:notAvailable",
    downloadProgress: "update:downloadProgress",
    downloaded: "update:downloaded",
  },
} as const;

// 统一响应信封：主进程所有 handler 都返回这个结构
export interface IpcResponse<T = unknown> {
  code: number;
  message: string;
  data: T | null;
}

// Dialog payload 契约
export interface DialogOpenOptions {
  type: "file" | "directory";
  multiple?: boolean;
  readContent?: boolean;
  filters?: Electron.FileFilter[];
}

export interface DialogSaveOptions {
  defaultPath?: string;
  content: string;
  filters?: Electron.FileFilter[];
}

// ContextMenu payload 契约
export interface ContextMenuItem {
  type?: "separator";
  key?: string;
  label?: string;
}

// 通道 → [参数, 返回值] 映射表，给 invoke 做类型推导
export interface InvokeMap {
  [IPC.dialog.open]: { args: [DialogOpenOptions]; return: string[] };
  [IPC.dialog.save]: { args: [DialogSaveOptions]; return: string };
  [IPC.shell.open]: { args: [string]; return: void };
  [IPC.contextMenu.show]: { args: [ContextMenuItem[]]; return: string | null };
  [IPC.update.check]: { args: []; return: void };
  [IPC.update.startDownload]: { args: []; return: void };
  [IPC.update.install]: { args: []; return: void };
}

export type InvokeChannel = keyof InvokeMap;

type EventNamespace = typeof EVENT[keyof typeof EVENT];
export type EventChannel = EventNamespace[keyof EventNamespace];
