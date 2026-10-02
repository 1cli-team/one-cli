import log from "electron-log/main";

export function createElectronLogger() {
  return log;
}
export type ElectronLogger = ReturnType<typeof createElectronLogger>;
