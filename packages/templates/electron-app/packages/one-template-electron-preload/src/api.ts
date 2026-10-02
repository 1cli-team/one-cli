import type { AppInfo } from "./channels";

export interface ElectronAPI {
  getAppInfo: () => Promise<AppInfo>;
}
