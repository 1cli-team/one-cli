import { contextBridge, ipcRenderer } from "electron";
import type { ElectronAPI } from "./api";
import { IPC } from "./channels";

const api: ElectronAPI = { getAppInfo: () => ipcRenderer.invoke(IPC.appInfo) };
contextBridge.exposeInMainWorld("electron", api);
export type { ElectronAPI } from "./api";
export type { AppInfo } from "./channels";
