import type { ElectronAPI } from "@one-template-electron/preload";
declare global {
  interface Window {
    electron?: ElectronAPI;
  }
}
export {};
