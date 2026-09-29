import type { ReactNode } from "react";
import { Search } from "lucide-react";
import { GithubIcon as Github } from "@/components/github-icon";
import Link from "next/link";
import { BrandMark } from "@/components/brand-mark";
import {
  localizedDocsPath,
  localizedTutorialsPath,
  type Locale,
} from "@/i18n";
import { LanguageSwitcher } from "@/components/language-switcher";

export type SiteTopNavActive = "docs" | "tutorials";

export function SiteTopNav({
  lang,
  active,
  sidebarTrigger,
}: {
  lang: Locale;
  active?: SiteTopNavActive;
  sidebarTrigger?: ReactNode;
}) {
  const labels = topNavText[lang];
  const navItems = [
    {
      key: "tutorials",
      label: labels.tutorials,
      href: localizedTutorialsPath(lang, ["templates"]),
    },
    {
      key: "docs",
      label: labels.docs,
      href: localizedDocsPath(lang, ["quick-start"]),
    },
  ] as const;

  return (
    <header className="one-docs-topbar">
      <Link href={`/${lang}/`} className="one-docs-brand" aria-label={labels.home}>
        <BrandMark variant="light" />
      </Link>
      <div className="one-docs-navigation">
        <nav className="one-docs-navlinks" aria-label={labels.navAria}>
          {navItems.map((item) => (
            <Link
              data-active={active === item.key}
              aria-current={active === item.key ? "page" : undefined}
              href={item.href}
              key={item.key}
            >
              {item.label}
            </Link>
          ))}
        </nav>
        {sidebarTrigger}
      </div>
      <div className="one-docs-topbar-right">
        <div className="one-docs-search" aria-hidden="true">
          <Search className="size-3.5" />
          <span>{labels.search}</span>
          <kbd>⌘K</kbd>
        </div>
        <LanguageSwitcher lang={lang} />
        <a
          href="https://github.com/1cli-team/one-cli"
          className="one-docs-icon-link"
          target="_blank"
          rel="noreferrer"
          aria-label="GitHub"
        >
          <Github className="size-[18px]" />
        </a>
      </div>
    </header>
  );
}

const topNavText: Record<
  Locale,
  {
    home: string;
    navAria: string;
    docs: string;
    tutorials: string;
    search: string;
  }
> = {
  zh: {
    home: "One CLI 首页",
    navAria: "站点导航",
    docs: "文档",
    tutorials: "教程",
    search: "搜索文档",
  },
  en: {
    home: "One CLI Home",
    navAria: "Site navigation",
    docs: "Docs",
    tutorials: "Tutorials",
    search: "Search docs",
  },
};
