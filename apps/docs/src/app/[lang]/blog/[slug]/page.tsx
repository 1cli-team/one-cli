import { ArrowLeft, CalendarDays, Clock3, FileText } from "lucide-react";
import Link from "next/link";
import { notFound } from "next/navigation";
import { MarkdownContent } from "@/components/markdown-content";
import { SiteTopNav } from "@/components/site-top-nav";
import {
  alternateBlogLanguages,
  htmlLang,
  isLocale,
  localizedBlogPath,
  type Locale,
} from "@/i18n";
import { formatBlogDate, getBlogPost, getBlogPosts } from "@/lib/blog";

const copy: Record<
  Locale,
  {
    back: string;
    readTime: string;
    editGithub: string;
  }
> = {
  zh: {
    back: "返回博客",
    readTime: "分钟阅读",
    editGithub: "在 GitHub 编辑",
  },
  en: {
    back: "Back to blog",
    readTime: "min read",
    editGithub: "Edit on GitHub",
  },
};

export function generateStaticParams() {
  return (["zh", "en"] as const).flatMap((lang) =>
    getBlogPosts(lang).map((post) => ({
      lang,
      slug: post.slug,
    })),
  );
}

export async function generateMetadata(props: {
  params: Promise<{ lang: string; slug: string }>;
}) {
  const { lang: rawLang, slug } = await props.params;
  if (!isLocale(rawLang)) notFound();
  const post = getBlogPost(rawLang, slug);
  if (!post) notFound();

  return {
    title: `${post.title} | One CLI Blog`,
    description: post.description,
    alternates: {
      canonical: localizedBlogPath(rawLang, [post.slug]),
      languages: alternateBlogLanguages([post.slug]),
    },
    other: {
      "content-language": htmlLang[rawLang],
    },
  };
}

export default async function BlogArticleRoute(props: {
  params: Promise<{ lang: string; slug: string }>;
}) {
  const { lang: rawLang, slug } = await props.params;
  if (!isLocale(rawLang)) notFound();
  const lang = rawLang;
  const post = getBlogPost(lang, slug);
  if (!post) notFound();
  const labels = copy[lang];

  return (
    <main className="min-h-screen bg-[var(--surface-primary)] text-[var(--foreground-primary)]">
      <SiteTopNav lang={lang} active="blog" standalone />
      <article className="mx-auto max-w-[900px] px-5 pt-28 pb-20 md:px-8">
        <Link
          className="no-style inline-flex items-center gap-2 text-sm font-semibold text-[var(--accent-primary)]"
          href={localizedBlogPath(lang)}
        >
          <ArrowLeft className="size-4" />
          {labels.back}
        </Link>
        <div className="mt-8 flex flex-wrap gap-2">
          {post.tags.map((tag) => (
            <span
              className="rounded-md border border-[var(--border-subtle)] bg-[var(--surface-elevated)] px-2.5 py-1 font-mono text-xs text-[var(--foreground-secondary)]"
              key={tag}
            >
              {tag}
            </span>
          ))}
        </div>
        <h1 className="mt-6 text-4xl font-bold leading-tight md:text-6xl">
          {post.title}
        </h1>
        <p className="mt-5 max-w-[760px] text-lg leading-8 text-[var(--foreground-secondary)]">
          {post.description}
        </p>
        <div className="one-docs-meta">
          <span>
            <CalendarDays className="size-3.5" />
            {formatBlogDate(lang, post.date)}
          </span>
          <span>
            <Clock3 className="size-3.5" />
            {post.readingTime} {labels.readTime}
          </span>
          <a
            href={`https://github.com/1cli-team/one-cli/blob/master/apps/docs/${post.contentPath}`}
            target="_blank"
            rel="noreferrer"
          >
            <FileText className="size-3.5" />
            {labels.editGithub}
          </a>
        </div>
        <div className="one-docs-divider" />
        <MarkdownContent content={post.content} />
      </article>
    </main>
  );
}
