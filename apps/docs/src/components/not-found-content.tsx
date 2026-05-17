"use client";

import { ArrowLeft, BookOpen } from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { SiteTopNav } from "@/components/site-top-nav";
import { defaultLocale, isLocale, type Locale, localizedDocsPath } from "@/i18n";

const copy: Record<
  Locale,
  {
    title: string;
    body: string;
    home: string;
    docs: string;
    hint: string;
  }
> = {
  zh: {
    title: "页面找不到了",
    body: "这个链接可能已经失效，或者你输错了。试试回首页或浏览文档。",
    home: "返回首页",
    docs: "浏览文档",
    hint: "错误代码",
  },
  en: {
    title: "Page not found",
    body: "This link may have expired, or it was mistyped. Try the home page or browse the docs.",
    home: "Back home",
    docs: "Browse docs",
    hint: "Error code",
  },
};

function resolveLocale(pathname: string | null): Locale {
  const first = pathname?.split("/").filter(Boolean)[0] ?? "";
  return isLocale(first) ? first : defaultLocale;
}

export function NotFoundContent() {
  const pathname = usePathname();
  const lang = resolveLocale(pathname);
  const text = copy[lang];

  return (
    <main className="min-h-screen bg-[var(--surface-primary)] text-[var(--foreground-primary)]">
      <SiteTopNav lang={lang} standalone />

      <section className="mx-auto flex max-w-3xl flex-col items-center px-6 pt-24 pb-32 text-center sm:pt-32">
        <span className="text-[11px] font-medium uppercase tracking-[0.32em] text-[var(--foreground-muted)]">
          {text.hint} · 404
        </span>

        <h1 className="mt-6 text-[clamp(6rem,18vw,11rem)] font-semibold leading-none tracking-tight text-[#ea580c]">
          404
        </h1>

        <h2 className="mt-8 text-2xl font-semibold tracking-tight text-[var(--foreground-primary)] sm:text-3xl">
          {text.title}
        </h2>

        <p className="mt-3 max-w-xl text-base text-[var(--foreground-secondary)] sm:text-lg">
          {text.body}
        </p>

        <div className="mt-10 flex flex-col items-center gap-3 sm:flex-row">
          <Link
            href={`/${lang}/`}
            className="inline-flex items-center gap-2 rounded-full bg-[#ea580c] px-5 py-2.5 text-sm font-medium text-white transition-colors hover:bg-[#c2410c]"
          >
            <ArrowLeft className="size-4" />
            {text.home}
          </Link>
          <Link
            href={localizedDocsPath(lang, ["quick-start"])}
            className="inline-flex items-center gap-2 rounded-full border border-[var(--border-subtle)] px-5 py-2.5 text-sm font-medium text-[var(--foreground-primary)] transition-colors hover:border-[var(--border-strong)] hover:bg-[var(--surface-elevated)]"
          >
            <BookOpen className="size-4" />
            {text.docs}
          </Link>
        </div>
      </section>
    </main>
  );
}
