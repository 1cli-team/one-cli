// 统一响应信封：所有 IPC 处理器返回值走这里
// preload 侧 code === 0 解包 data；非 0 抛 Error(message)
export interface IpcResponse {
  code: number;
  message: string;
  data: Record<string, any> | null;
}

export function success(data: unknown): IpcResponse {
  return { code: 0, message: "success", data: (data ?? null) as Record<string, any> | null };
}

export function error(message = "fail"): IpcResponse {
  return { code: -1, message, data: null };
}
