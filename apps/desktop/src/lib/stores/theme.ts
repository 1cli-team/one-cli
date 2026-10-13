import { createStore } from "@/lib/utils";

export type ThemeMode = "light" | "dark";

interface ThemeState {
	mode: ThemeMode;
	setMode: (m: ThemeMode) => void;
	toggle: () => void;
}

const THEME_KEY = "app_theme_mode";

const getInitial = (): ThemeMode => {
	const cached = localStorage.getItem(THEME_KEY) as ThemeMode | null;
	if (cached) return cached;
	// 跟随系统
	return window.matchMedia?.("(prefers-color-scheme: dark)").matches ? "dark" : "light";
};

export const useThemeStore = createStore<ThemeState>(
	(set, get) => ({
		mode: getInitial(),
		setMode: (m) => {
			localStorage.setItem(THEME_KEY, m);
			set({ mode: m });
		},
		toggle: () => {
			const next = get().mode === "light" ? "dark" : "light";
			localStorage.setItem(THEME_KEY, next);
			set({ mode: next });
		},
	}),
	"themeStore",
);
