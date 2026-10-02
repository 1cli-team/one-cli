export const IPC = { appInfo: "app.info" } as const;
export interface AppInfo {
  name: string;
  version: string;
  platform: string;
}
