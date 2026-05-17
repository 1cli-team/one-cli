import { ArrowRight, CalendarDays, Clock3 } from "lucide-react";
import Link from "next/link";
import { notFound } from "next/navigation";
import { SiteTopNav } from "@/components/site-top-nav";
import {
  alternateBlogLanguages,
  htmlLang,
  isLocale,
  localizedBlogPath,
  type Locale,
} from "@/i18n";
import { formatBlogDate, getBlogPosts } from "@/lib/blog";

const copy: Record<
  Locale,
  {
    eyebrow: string;
    title: string;
    description: string;
    read: string;
    empty: string;
  }
> = {
  zh: {
    eyebrow: "One CLI Blog",
    title: "工程笔记和产品设计记录",
    description:
      "记录 One CLI 的 manifest、preset、agent skill 和模板治理设计，方便后续决策有上下文可追溯。",
    read: "阅读全文",
    empty: "还没有博客文章。",
  },
  en: {
    eyebrow: "One CLI Blog",
    title: "Engineering notes and product design records",
    description:
      "Notes on One CLI manifests, presets, agent skills, and template governance so decisions stay traceable.",
    read: "Read article",
    empty: "No blog posts yet.",
  },
};

export function generateStaticParams() {
  return [{ lang: "zh" }, { lang: "en" }];
}

export async function generateMetadata(props: {
  params: Promise<{ lang: string }>;
}) {
  const { lang: rawLang } = await props.params;
  if (!isLocale(rawLang)) notFound();
  const lang = rawLang;

  return {
    title: lang === "zh" ? "博客 | One CLI" : "Blog | One CLI",
    description: copy[lang].description,
    alternates: {
      canonical: localizedBlogPath(lang),
      languages: alternateBlogLanguages(),
    },
    other: {
      "content-language": htmlLang[lang],
    },
  };
}

export default async function BlogIndexRoute(props: {
  params: Promise<{ lang: string }>;
}) {
  const { lang: rawLang } = await props.params;
  if (!isLocale(rawLang)) notFound();
  const lang = rawLang;
  const text = copy[lang];
  const posts = getBlogPosts(lang);

  return (
    <main className="min-h-screen bg-[var(--surface-primary)] text-[var(--foreground-primary)]">
      <SiteTopNav lang={lang} active="blog" standalone />
      <section className="border-b border-[var(--border-subtle)] bg-[var(--surface-elevated)] px-5 pt-28 pb-16 lg:px-24">
        <div className="mx-auto max-w-[1120px]">
          <p className="font-mono text-xs font-semibold uppercase text-[var(--accent-primary)]">
            {text.eyebrow}
          </p>
          <h1 className="mt-4 max-w-[780px] text-4xl font-bold leading-tight md:text-6xl">
            {text.title}
          </h1>
          <p className="mt-5 max-w-[720px] text-base leading-8 text-[var(--foreground-secondary)]">
            {text.description}
          </p>
        </div>
      </section>
      <section className="px-5 py-12 lg:px-24 lg:py-16">
        <div className="mx-auto grid max-w-[1120px] gap-5 md:grid-cols-2">
          {posts.length === 0 ? (
            <p className="text-sm text-[var(--foreground-secondary)]">{text.empty}</p>
          ) : (
            posts.map((post) => (
              <Link
                className="group rounded-lg border border-[var(--border-subtle)] bg-white p-6 text-inherit no-underline transition hover:border-[var(--accent-border)] hover:bg-[var(--accent-soft)]"
                href={localizedBlogPath(lang, [post.slug])}
                key={post.slug}
              >
                <div className="flex flex-wrap gap-2">
                  {post.tags.map((tag) => (
                    <span
                      className="rounded-md border border-[var(--border-subtle)] bg-[var(--surface-elevated)] px-2.5 py-1 font-mono text-xs text-[var(--foreground-secondary)]"
                      key={tag}
                    >
                      {tag}
                    </span>
                  ))}
                </div>
                <h2 className="mt-5 text-2xl font-semibold leading-snug">
                  {post.title}
                </h2>
                <p className="mt-3 text-sm leading-6 text-[var(--foreground-secondary)]">
                  {post.description}
                </p>
                <div className="mt-6 flex flex-wrap items-center gap-4 text-xs text-[var(--foreground-muted)]">
                  <span className="inline-flex items-center gap-1.5">
                    <CalendarDays className="size-3.5" />
                    {formatBlogDate(lang, post.date)}
                  </span>
                  <span className="inline-flex items-center gap-1.5">
                    <Clock3 className="size-3.5" />
                    {post.readingTime} min
                  </span>
                </div>
                <span className="mt-6 inline-flex items-center gap-2 text-sm font-semibold text-[var(--accent-primary)]">
                  {text.read}
                  <ArrowRight className="size-4 transition group-hover:translate-x-0.5" />
                </span>
              </Link>
            ))
          )}
        </div>
      </section>
    </main>
  );
}
