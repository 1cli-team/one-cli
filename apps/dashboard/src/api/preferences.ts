// Thin client over /api/preferences. Mirrors api/configure.ts's style
// (HttpClient + typed result). Used by the locale store to read the
// initial preference and persist user choices so the CLI and the
// dashboard share one source of truth.

import { http } from "@/lib/http";

export type StoredLocale = "auto" | "zh-CN" | "en-US";

export interface PreferencesPayload {
	schema: string;
	stored_locale: StoredLocale;
	resolved: "zh-CN" | "en-US";
	detected?: string;
	config_path: string;
}

// http baseURL is already "/api"; pass the path under that prefix.
export async function getPreferences(): Promise<PreferencesPayload> {
	return http.get<PreferencesPayload>("/preferences");
}

export async function putLocale(locale: StoredLocale): Promise<PreferencesPayload> {
	return http.put<PreferencesPayload>("/preferences", { locale });
}
