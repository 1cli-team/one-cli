"use client";

import { Github, Menu } from "lucide-react";
import Link from "next/link";
import { BrandMark } from "@/components/brand-mark";
import { localeLabels, locales, type Locale } from "@/i18n";

type NavLocale = "zh" | "en";

export type NavText = {
  docs: string;
  tutorials: string;
  templates: string;
  blog: string;
  changelog: string;
  getStarted: string;
  homeAria: string;
  languageSwitcher: string;
  openMenu: string;
};

export const navText: Record<NavLocale, NavText> = {
  zh: {
    docs: "文档",
    tutorials: "教程",
    templates: "模板",
    blog: "博客",
    changelog: "更新日志",
    getStarted: "开始使用",
    homeAria: "One CLI 首页",
    languageSwitcher: "语言切换",
    openMenu: "打开菜单",
  },
  en: {
    docs: "Docs",
    tutorials: "Tutorials",
    templates: "Templates",
    blog: "Blog",
    changelog: "Changelog",
    getStarted: "Get started",
    homeAria: "One CLI home",
    languageSwitcher: "Language switcher",
    openMenu: "Open menu",
  },
};

export function TemplateExamplesNav({ lang }: { lang: NavLocale }) {
  const text = navText[lang];
  const prefix = `/${lang}`;

  return (
    <header className="sticky top-0 z-30 border-b border-stone-200 bg-white/92 backdrop-blur-xl">
      <div className="mx-auto flex h-16 w-full max-w-[1440px] items-center justify-between gap-4 px-5 lg:px-24">
        <div className="flex items-center gap-9">
          <Link
            href={`${prefix}/`}
            className="inline-flex items-center"
            aria-label={text.homeAria}
          >
            <BrandMark variant="light" />
          </Link>
          <nav className="hidden items-center gap-7 text-sm lg:flex">
            <Link
              href={`${prefix}/tutorials/templates/`}
              className="text-stone-600 hover:text-[#0a0a0a]"
            >
              {text.tutorials}
            </Link>
            <Link
              href={`${prefix}/docs/quick-start/`}
              className="text-stone-600 hover:text-[#0a0a0a]"
            >
              {text.docs}
            </Link>
            <Link
              href={`${prefix}/templates/`}
              className="font-semibold text-[#0a0a0a]"
            >
              {text.templates}
            </Link>
            <Link
              href={`${prefix}/blog/`}
              className="text-stone-600 hover:text-[#0a0a0a]"
            >
              {text.blog}
            </Link>
            <a
              href="https://github.com/1cli-team/one-cli/blob/master/CHANGELOG.md"
              target="_blank"
              rel="noreferrer"
              className="text-stone-600 hover:text-[#0a0a0a]"
            >
              {text.changelog}
            </a>
          </nav>
        </div>
        <div className="flex items-center gap-3">
          <div
            className="one-docs-lang-switcher"
            aria-label={text.languageSwitcher}
          >
            {locales.map((locale) => (
              <Link
                aria-current={locale === lang ? "true" : undefined}
                data-active={locale === lang}
                href={`/${locale}/templates/`}
                key={locale}
              >
                {localeLabels[locale as Locale]}
              </Link>
            ))}
          </div>
          <a
            href="https://github.com/1cli-team/one-cli"
            target="_blank"
            rel="noreferrer"
            className="hidden text-stone-700 hover:text-[#0a0a0a] sm:inline-flex"
            aria-label="GitHub"
          >
            <Github className="size-5" />
          </a>
          <Link
            href={`${prefix}/docs/installation/`}
            className="hidden h-9 items-center rounded-md bg-[#ea580c] px-3.5 text-sm font-semibold text-white hover:bg-[#c2410c] sm:inline-flex"
          >
            {text.getStarted}
          </Link>
          <button
            type="button"
            className="inline-flex size-9 items-center justify-center rounded-md border border-stone-200 text-stone-700 lg:hidden"
            aria-label={text.openMenu}
          >
            <Menu className="size-5" />
          </button>
        </div>
      </div>
    </header>
  );
}
