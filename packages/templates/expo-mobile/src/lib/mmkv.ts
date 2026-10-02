import { createMMKV } from "react-native-mmkv";
import type { StateStorage } from "zustand/middleware";

const storage = createMMKV({ id: "ui-preferences" });
export const preferenceStorage: StateStorage = {
  getItem: (name) => storage.getString(name) ?? null,
  setItem: (name, value) => storage.set(name, value),
  removeItem: (name) => {
    storage.remove(name);
  },
};
