import { create } from "zustand";
import { createJSONStorage, persist } from "zustand/middleware";

type ThemeMode = "light" | "dark";
interface ThemeState {
	mode: ThemeMode;
	toggle: () => void;
}
export const useThemeStore = create<ThemeState>()(
	persist(
		(set) => ({
			mode: window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light",
			toggle: () => set((state) => ({ mode: state.mode === "dark" ? "light" : "dark" })),
		}),
		{
			name: "ui-theme-v1",
			storage: createJSONStorage(() => localStorage),
			partialize: (state) => ({ mode: state.mode }),
		},
	),
);
