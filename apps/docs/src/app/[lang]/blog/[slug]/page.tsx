import { ArrowLeft, CalendarDays, Clock3, FileText } from "lucide-react";
import Link from "next/link";
import { notFound } from "next/navigation";
import { MarkdownContent } from "@/components/markdown-content";
import { SiteTopNav } from "@/components/site-top-nav";
import {
  alternateBlogLanguages,
  isLocale,
  localizedBlogPath,
  type Locale,
} from "@/i18n";
import { formatBlogDate, getBlogPost, getBlogPosts } from "@/lib/blog";
import {
  blogPostingJsonLd,
  breadcrumbJsonLd,
  createPageMetadata,
  jsonLdScriptProps,
} from "@/lib/seo";

const copy: Record<
  Locale,
  {
    back: string;
    readTime: string;
    editGithub: string;
    related: string;
  }
> = {
  zh: {
    back: "返回博客",
    readTime: "分钟阅读",
    editGithub: "在 GitHub 编辑",
    related: "继续阅读",
  },
  en: {
    back: "Back to blog",
    readTime: "min read",
    editGithub: "Edit on GitHub",
    related: "Related reading",
  },
};

const agenticEngineeringSeoPatch = {
  metadataTitle: "Agentic Engineering | 1cli",
  canonicalUrl: "https://www.1cli.dev/en/blog/agentic-engineering/",
  webPageJsonLd: {
    "@context": "https://schema.org",
    "@type": "WebPage",
    description:
      "Learn about Agentic Engineering, including key details, use cases, and next steps.",
    name: "Agentic Engineering | 1cli",
    url: "https://www.1cli.dev/en/blog/agentic-engineering/",
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
  const seoPatch = getBlogSeoPatch(rawLang, post.slug);

  return createPageMetadata({
    title: seoPatch?.metadataTitle ?? `${post.title} | One CLI Blog`,
    description: post.description,
    path: seoPatch?.canonicalUrl ?? localizedBlogPath(rawLang, [post.slug]),
    locale: rawLang,
    alternates: alternateBlogLanguages([post.slug]),
    type: "article",
  });
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
  const related = getRelatedPosts(lang, post.slug, post.tags);
  const seoPatch = getBlogSeoPatch(lang, post.slug);
  const articlePath = seoPatch?.canonicalUrl ?? localizedBlogPath(lang, [post.slug]);

  return (
    <main className="min-h-screen bg-[var(--surface-primary)] text-[var(--foreground-primary)]">
      {seoPatch ? <script {...jsonLdScriptProps(seoPatch.webPageJsonLd)} /> : null}
      <script
        {...jsonLdScriptProps([
          blogPostingJsonLd({
            title: post.title,
            description: post.description,
            path: articlePath,
            locale: lang,
            datePublished: post.date,
            author: post.author,
            tags: post.tags,
            section: "One CLI Blog",
          }),
          breadcrumbJsonLd([
            { name: "One CLI", path: `/${lang}/` },
            { name: "Blog", path: localizedBlogPath(lang) },
            { name: post.title, path: articlePath },
          ]),
        ])}
      />
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
        {related.length > 0 ? (
          <section className="mt-14 border-t border-[var(--border-subtle)] pt-8">
            <h2 className="text-xl font-semibold">{labels.related}</h2>
            <div className="mt-5 grid gap-3">
              {related.map((item) => (
                <Link
                  className="no-style rounded-lg border border-[var(--border-subtle)] bg-[var(--surface-elevated)] p-4 transition hover:border-[var(--accent-border)] hover:bg-[var(--accent-soft)]"
                  href={localizedBlogPath(lang, [item.slug])}
                  key={item.slug}
                >
                  <span className="text-base font-semibold text-[var(--foreground-primary)]">
                    {item.title}
                  </span>
                  <span className="mt-1 block text-sm leading-6 text-[var(--foreground-secondary)]">
                    {item.description}
                  </span>
                </Link>
              ))}
            </div>
          </section>
        ) : null}
      </article>
    </main>
  );
}

function getBlogSeoPatch(lang: Locale, slug: string) {
  return lang === "en" && slug === "agentic-engineering"
    ? agenticEngineeringSeoPatch
    : null;
}

function getRelatedPosts(lang: Locale, slug: string, tags: string[]) {
  const tagSet = new Set(tags);

  return getBlogPosts(lang)
    .filter((candidate) => candidate.slug !== slug)
    .map((candidate) => ({
      candidate,
      score: candidate.tags.filter((tag) => tagSet.has(tag)).length,
    }))
    .sort(
      (a, b) =>
        b.score - a.score || b.candidate.date.localeCompare(a.candidate.date),
    )
    .slice(0, 3)
    .map(({ candidate }) => candidate);
}
