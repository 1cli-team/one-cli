// I18nProvider wires the locale store to three side-effects:
//
//   1. Set i18next's active language so useTranslation() returns
//      the right catalog.
//   2. Set <html lang="..."> so the browser, screen readers, and
//      any future printable surface know the page language.
//   3. POST the user's choice to /api/preferences so the CLI side
//      stays in sync — same preferences.json drives both surfaces.
//
// On first mount we ALSO read /api/preferences and, if no local
// override was stored, adopt whatever the CLI thinks. This makes the
// "first time the dashboard opens after a `one configure locale`"
// case work without the user noticing the round-trip.

import { type ReactNode, useEffect, useRef } from "react";
import i18n from "@/lib/i18n";
import { useLocaleStore } from "@/lib/stores/locale";
import { getPreferences, putLocale } from "@/api/preferences";
import { hasToken } from "@/lib/http";

interface I18nProviderProps {
	children: ReactNode;
}

const STORAGE_KEY = "app_locale_mode";

export function I18nProvider({ children }: I18nProviderProps) {
	const { mode, resolved, setMode } = useLocaleStore();

	// Bootstrap: only run once. If the user has no localStorage
	// override, fetch the CLI preference and adopt it.
	const bootstrappedRef = useRef(false);
	useEffect(() => {
		if (bootstrappedRef.current) return;
		bootstrappedRef.current = true;
		if (!hasToken()) return; // no API access; just stick with local default
		const hasLocalOverride =
			typeof localStorage !== "undefined" && localStorage.getItem(STORAGE_KEY) != null;
		if (hasLocalOverride) return;
		void (async () => {
			try {
				const p = await getPreferences();
				if (p.stored_locale === "auto" || p.stored_locale === "zh-CN" || p.stored_locale === "en-US") {
					setMode(p.stored_locale);
				}
			} catch {
				// API may be unreachable in --no-ui dev mode; ignore and
				// fall back to whatever the local default produced.
			}
		})();
	}, [setMode]);

	// Apply resolved locale to i18next + <html lang>.
	useEffect(() => {
		void i18n.changeLanguage(resolved);
		if (typeof document !== "undefined") {
			document.documentElement.lang = resolved;
		}
	}, [resolved]);

	// Persist the *stored* mode (what the user picked) to /api/preferences
	// so the CLI sees the same thing. Skipped on initial mount because the
	// bootstrap above might have just adopted the same value — avoids a
	// pointless round-trip. Skipped also when token is missing.
	const lastSentRef = useRef<string | null>(null);
	useEffect(() => {
		if (!hasToken()) return;
		if (lastSentRef.current === mode) return;
		lastSentRef.current = mode;
		void putLocale(mode).catch(() => {
			// Network blip — the local choice still works; the CLI just
			// won't pick it up until next time. Silent fail is OK.
		});
	}, [mode]);

	return <>{children}</>;
}
