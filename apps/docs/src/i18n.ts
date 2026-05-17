import { defineI18n } from "fumadocs-core/i18n";

export const locales = ["zh", "en"] as const;
export type Locale = (typeof locales)[number];

export const defaultLocale: Locale = "zh";

export const i18n = defineI18n({
  languages: [...locales],
  defaultLanguage: defaultLocale,
  hideLocale: "never",
  parser: "dir",
  fallbackLanguage: defaultLocale,
});

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

export function localizedDocsPath(locale: Locale, slug?: string[]) {
  const suffix = slug && slug.length > 0 ? `/${slug.join("/")}` : "";
  return `/${locale}/docs${suffix}/`;
}

export function localizedBlogPath(locale: Locale, slug?: string[]) {
  const suffix = slug && slug.length > 0 ? `/${slug.join("/")}` : "";
  return `/${locale}/blog${suffix}/`;
}

export function localizedTutorialsPath(locale: Locale, slug?: string[]) {
  const suffix = slug && slug.length > 0 ? `/${slug.join("/")}` : "";
  return `/${locale}/tutorials${suffix}/`;
}

export function alternateDocsLanguages(slug?: string[]) {
  return {
    "zh-Hans": localizedDocsPath("zh", slug),
    en: localizedDocsPath("en", slug),
    "x-default": localizedDocsPath(defaultLocale, slug),
  };
}

export function alternateBlogLanguages(slug?: string[]) {
  return {
    "zh-Hans": localizedBlogPath("zh", slug),
    en: localizedBlogPath("en", slug),
    "x-default": localizedBlogPath(defaultLocale, slug),
  };
}

export function alternateTutorialsLanguages(slug?: string[]) {
  return {
    "zh-Hans": localizedTutorialsPath("zh", slug),
    en: localizedTutorialsPath("en", slug),
    "x-default": localizedTutorialsPath(defaultLocale, slug),
  };
}
