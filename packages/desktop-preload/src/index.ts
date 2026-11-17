import { contextBridge, ipcRenderer } from "electron";
import type { DesktopAPI } from "./api";

const api: DesktopAPI = {
  getLegacyPreferences: () => ipcRenderer.invoke("desktop:legacy-preferences"),
  getBackendConnection: () => ipcRenderer.invoke("desktop:connection"),
  onBackendState(callback) {
    const listener = () => callback();
    ipcRenderer.on("desktop:state", listener);
    return () => ipcRenderer.removeListener("desktop:state", listener);
  },
  retryBackend: () => ipcRenderer.invoke("desktop:retry"),
  confirmHealth: (generation) => ipcRenderer.invoke("desktop:health", generation),
  chooseWorkspace: () => ipcRenderer.invoke("desktop:choose-workspace"),
  openExternal: (url) => ipcRenderer.invoke("desktop:open-external", url),
  setNativeLocale: (locale) => ipcRenderer.invoke("desktop:locale", locale),
  checkUpdate: () => ipcRenderer.invoke("desktop:check-update"),
  installUpdate: () => ipcRenderer.invoke("desktop:install-update"),
};

contextBridge.exposeInMainWorld("desktop", api);
export type { DesktopAPI, Connection, UpdateInfo } from "./api";
