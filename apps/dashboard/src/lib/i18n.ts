// i18next bootstrap. Resources are imported statically so they bundle
// into the JS chunks Vite emits — no async loader, no Suspense
// boundary needed at app start. Switching language uses
// i18n.changeLanguage; the locale store wires that to user actions
// and to /api/preferences persistence.

import i18n from "i18next";
import { initReactI18next } from "react-i18next";
import enUS from "@/locales/en-US.json";
import zhCN from "@/locales/zh-CN.json";

export type SupportedLocale = "zh-CN" | "en-US";

export const DEFAULT_LOCALE: SupportedLocale = "en-US";

// detectBrowserLocale maps navigator.language onto one of the
// supported tags. Mirrors the Go-side detect.go logic (zh* → zh-CN,
// everything else → en-US) so the dashboard's auto-mode and the
// CLI's auto-mode produce the same result on the same machine.
export function detectBrowserLocale(): SupportedLocale {
	const raw = typeof navigator !== "undefined" ? navigator.language ?? "" : "";
	if (raw.toLowerCase().startsWith("zh")) return "zh-CN";
	return "en-US";
}

void i18n
	.use(initReactI18next)
	.init({
		resources: {
			"zh-CN": { translation: zhCN },
			"en-US": { translation: enUS },
		},
		lng: DEFAULT_LOCALE,
		fallbackLng: DEFAULT_LOCALE,
		interpolation: { escapeValue: false },
		// react-i18next v16 expects keys with `_one`/`_other` suffixes
		// when JSON-style plurals are used; this matches our locale files.
		returnEmptyString: false,
	});

export default i18n;
