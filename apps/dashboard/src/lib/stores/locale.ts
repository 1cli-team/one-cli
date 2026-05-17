// Locale preference store. Mirrors lib/stores/theme.ts: a tiny
// Zustand store that persists the user's choice to localStorage and
// exposes set + cycle actions.
//
// Two values: a stored mode (what the user picked) and a resolved
// locale (what i18next is actually rendering). Auto mode means
// "follow either the saved CLI preference or the browser locale";
// zh-CN / en-US force a specific catalog.

import { detectBrowserLocale, type SupportedLocale } from "@/lib/i18n";
import { createStore } from "@/lib/utils";

export type LocaleMode = "auto" | "zh-CN" | "en-US";

const STORAGE_KEY = "app_locale_mode";

interface LocaleState {
	mode: LocaleMode;
	resolved: SupportedLocale;
	// setMode is the user-action entry point. It writes to localStorage
	// AND triggers I18nProvider's reactive subscription, which then
	// updates i18next + PUTs /api/preferences in one place.
	setMode: (m: LocaleMode) => void;
	// setResolved is provided so the bootstrap path can sync the
	// fetched /api/preferences result back into the store without
	// triggering a write (avoids a feedback loop).
	setResolved: (r: SupportedLocale) => void;
}

function readStored(): LocaleMode {
	if (typeof localStorage === "undefined") return "auto";
	const v = localStorage.getItem(STORAGE_KEY);
	if (v === "auto" || v === "zh-CN" || v === "en-US") return v;
	return "auto";
}

function resolveFromMode(mode: LocaleMode): SupportedLocale {
	if (mode === "zh-CN" || mode === "en-US") return mode;
	return detectBrowserLocale();
}

export const useLocaleStore = createStore<LocaleState>(
	(set) => {
		const initial = readStored();
		return {
			mode: initial,
			resolved: resolveFromMode(initial),
			setMode: (m) => {
				if (typeof localStorage !== "undefined") {
					localStorage.setItem(STORAGE_KEY, m);
				}
				set({ mode: m, resolved: resolveFromMode(m) });
			},
			setResolved: (r) => set({ resolved: r }),
		};
	},
	"localeStore",
);
