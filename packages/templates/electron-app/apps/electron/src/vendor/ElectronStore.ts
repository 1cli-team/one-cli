import Store from "electron-store";
import { workspace } from "../utils";

interface AppPersistentStore {
  mainBounds?: Electron.Rectangle;
}

export function createElectronStore() {
  return new Store<AppPersistentStore>({
    name: "window-state",
    cwd: workspace,
    defaults: {},
  });
}

export type ElectronStore = ReturnType<typeof createElectronStore>;
