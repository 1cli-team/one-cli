export const locales = ["zh", "en"] as const;
export type Locale = (typeof locales)[number];

export const defaultLocale: Locale = "zh";

export const localeLabels: Record<Locale, string> = {
	zh: "中文",
	en: "English",
};

export const htmlLang: Record<Locale, string> = {
	zh: "zh-Hans",
	en: "en",
};

export function isLocale(value: string): value is Locale {
	return locales.includes(value as Locale);
}
