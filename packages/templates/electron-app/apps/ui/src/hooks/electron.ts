import type { ElectronAPI } from "@app/preload";

// 直接透传 preload 暴露的 API
// preload 内部已解包信封并在失败时抛错，这里不再重复包装
export default function useElectron(): ElectronAPI {
  return window.electron;
}
