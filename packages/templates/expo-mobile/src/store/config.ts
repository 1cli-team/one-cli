import { create } from "zustand";
import { createJSONStorage, persist } from "zustand/middleware";
import { preferenceStorage } from "@/lib/mmkv";

type Theme = "system" | "light" | "dark";
interface ConfigState {
  theme: Theme;
  setTheme: (theme: Theme) => void;
}
export const useConfigStore = create<ConfigState>()(
  persist(
    (set) => ({
      theme: "system",
      setTheme: (theme) => set({ theme }),
    }),
    {
      name: "ui-preferences-v1",
      storage: createJSONStorage(() => preferenceStorage),
      partialize: (state) => ({ theme: state.theme }),
    },
  ),
);
