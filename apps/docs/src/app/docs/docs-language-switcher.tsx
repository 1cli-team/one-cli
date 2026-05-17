"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { localeLabels, locales, type Locale } from "@/i18n";

export function DocsLanguageSwitcher({ lang }: { lang: Locale }) {
  const pathname = usePathname();

  return (
    <div className="one-docs-lang-switcher" aria-label="Language switcher">
      {locales.map((locale) => (
        <Link
          aria-current={locale === lang ? "true" : undefined}
          data-active={locale === lang}
          href={switchLocale(pathname, locale)}
          key={locale}
        >
          {localeLabels[locale]}
        </Link>
      ))}
    </div>
  );
}

function switchLocale(pathname: string, nextLocale: Locale) {
  if (/^\/(zh|en)(\/|$)/.test(pathname)) {
    return pathname.replace(/^\/(zh|en)(?=\/|$)/, `/${nextLocale}`);
  }

  return `/${nextLocale}${pathname === "/" ? "" : pathname}`;
}
