import { Github, Search } from "lucide-react";
import Link from "next/link";
import { BrandMark } from "@/components/brand-mark";
import {
  localizedBlogPath,
  localizedDocsPath,
  localizedTutorialsPath,
  type Locale,
} from "@/i18n";
import { DocsLanguageSwitcher } from "@/app/docs/docs-language-switcher";

type SiteTopNavActive = "docs" | "tutorials" | "templates" | "blog";

export function SiteTopNav({
  lang,
  active,
  standalone = false,
}: {
  lang: Locale;
  active?: SiteTopNavActive;
  standalone?: boolean;
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
    {
      key: "templates",
      label: labels.templates,
      href: `/${lang}/templates/`,
    },
    {
      key: "blog",
      label: labels.blog,
      href: localizedBlogPath(lang),
    },
  ] as const;

  return (
    <header className="one-docs-topbar" data-standalone={standalone ? "true" : undefined}>
      <div className="one-docs-topbar-left">
        <Link href={`/${lang}/`} className="inline-flex items-center" aria-label={labels.home}>
          <BrandMark variant="light" />
        </Link>
        <nav className="one-docs-navlinks" aria-label={labels.navAria}>
          {navItems.map((item) => (
            <Link
              data-active={active === item.key}
              href={item.href}
              key={item.key}
            >
              {item.label}
            </Link>
          ))}
          <a
            href="https://github.com/torchstellar-team/one-cli/blob/master/CHANGELOG.md"
            target="_blank"
            rel="noreferrer"
          >
            {labels.changelog}
          </a>
        </nav>
      </div>
      <div className="one-docs-topbar-right">
        <div className="one-docs-search" aria-hidden="true">
          <Search className="size-3.5" />
          <span>{labels.search}</span>
          <kbd>⌘K</kbd>
        </div>
        <DocsLanguageSwitcher lang={lang} />
        <a
          href="https://github.com/torchstellar-team/one-cli"
          className="one-docs-icon-link"
          target="_blank"
          rel="noreferrer"
          aria-label="GitHub"
        >
          <Github className="size-[18px]" />
        </a>
        <Link
          href={localizedDocsPath(lang, ["installation"])}
          className="one-docs-start"
        >
          {labels.start}
        </Link>
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
    templates: string;
    blog: string;
    changelog: string;
    search: string;
    start: string;
  }
> = {
  zh: {
    home: "One CLI 首页",
    navAria: "站点导航",
    docs: "文档",
    tutorials: "教程",
    templates: "模板",
    blog: "博客",
    changelog: "更新日志",
    search: "搜索文档",
    start: "开始使用",
  },
  en: {
    home: "One CLI Home",
    navAria: "Site navigation",
    docs: "Docs",
    tutorials: "Tutorials",
    templates: "Templates",
    blog: "Blog",
    changelog: "Changelog",
    search: "Search docs",
    start: "Get started",
  },
};
