"use client";

import { Menu } from "@base-ui/react/menu";
import { Check, Languages } from "lucide-react";
import { Link } from "next-view-transitions";
import { usePathname } from "next/navigation";
import { htmlLang, localeLabels, locales, type Locale } from "@/i18n";

export function LanguageSwitcher({
  lang,
  variant = "light",
}: {
  lang: Locale;
  variant?: "light" | "dark";
}) {
  const pathname = usePathname();
  const label = lang === "zh" ? "切换语言" : "Change language";

  return (
    <Menu.Root key={pathname} modal={false}>
      <Menu.Trigger
        className="one-language-trigger"
        data-variant={variant}
        aria-label={`${label}: ${localeLabels[lang]}`}
      >
        <Languages aria-hidden="true" className="size-[18px]" />
      </Menu.Trigger>
      <Menu.Portal>
        <Menu.Positioner align="end" sideOffset={8} className="z-[60]">
          <Menu.Popup className="one-language-menu" data-variant={variant} aria-label={label}>
            {locales.map((locale) => (
              <Menu.Item
                key={locale}
                className="one-language-option"
                data-active={locale === lang}
                render={
                  <Link
                    href={switchLocale(pathname, locale)}
                    hrefLang={htmlLang[locale]}
                    lang={htmlLang[locale]}
                    aria-current={locale === lang ? "true" : undefined}
                  />
                }
              >
                <span>{localeLabels[locale]}</span>
                {locale === lang && <Check aria-hidden="true" className="size-3.5" />}
              </Menu.Item>
            ))}
          </Menu.Popup>
        </Menu.Positioner>
      </Menu.Portal>
    </Menu.Root>
  );
}

function switchLocale(pathname: string, nextLocale: Locale) {
  if (/^\/(zh|en)(\/|$)/.test(pathname)) {
    return pathname.replace(/^\/(zh|en)(?=\/|$)/, `/${nextLocale}`);
  }

  return `/${nextLocale}${pathname === "/" ? "/" : pathname}`;
}
