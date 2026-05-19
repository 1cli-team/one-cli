import { tutorialsSource } from "@/lib/source";
import {
  DocsPage,
  DocsBody,
  DocsTitle,
  DocsDescription,
} from "fumadocs-ui/page";
import { ChevronRight, Clock, FileText, GitCommit } from "lucide-react";
import { notFound } from "next/navigation";
import { isValidElement, type ReactNode } from "react";
import {
  alternateTutorialsLanguages,
  isLocale,
  localizedDocsPath,
  localizedTutorialsPath,
  type Locale,
} from "@/i18n";
import { DocsToc } from "../../../docs/docs-toc";
import { VideoEmbed } from "@/components/video-embed";
import {
  articleJsonLd,
  breadcrumbJsonLd,
  createPageMetadata,
  jsonLdScriptProps,
} from "@/lib/seo";

export default async function Page(props: {
  params: Promise<{ lang: string; slug?: string[] }>;
}) {
  const params = await props.params;
  if (!isLocale(params.lang)) notFound();
  const lang = params.lang;
  const page = tutorialsSource.getPage(params.slug, lang);
  if (!page) notFound();

  const data = page.data as unknown as {
    title: string;
    description?: string;
    body: React.ComponentType<{
      components?: Record<string, React.ComponentType<unknown>>;
    }>;
    toc?: import("fumadocs-core/server").TableOfContents;
    full?: boolean;
    kind?: string;
  };
  const MDX = data.body;
  const docPath = getContentPath(lang, params.slug);
  const toc = (data.toc ?? []).map((item) => ({
    ...item,
    title: tocTitleToText(item.title),
  }));
  const isVideo = data.kind === "video";
  const tutorialUrl = localizedTutorialsPath(lang, params.slug);

  return (
    <DocsPage
      toc={toc}
      full={data.full}
      breadcrumb={{
        component: (
          <ArticleBreadcrumb
            lang={lang}
            slug={params.slug}
            title={String(data.title)}
          />
        ),
      }}
      container={{ className: "one-docs-page" }}
      article={{ id: "nd-page_article", className: "one-docs-article" }}
      tableOfContent={{
        component: <DocsToc docPath={docPath} items={toc} lang={lang} />,
      }}
      tableOfContentPopover={{ enabled: false }}
    >
      <script
        {...jsonLdScriptProps([
          articleJsonLd({
            title: data.title,
            description: data.description,
            path: tutorialUrl,
            locale: lang,
            section: isVideo ? "Video tutorial" : "Tutorial",
          }),
          breadcrumbJsonLd([
            { name: "One CLI", path: `/${lang}/` },
            { name: uiText[lang].tutorials, path: localizedTutorialsPath(lang) },
            { name: String(data.title), path: tutorialUrl },
          ]),
        ])}
      />
      <DocsTitle className="one-docs-title">{data.title}</DocsTitle>
      <DocsDescription className="one-docs-description">
        {data.description}
      </DocsDescription>
      <ArticleMeta docPath={docPath} lang={lang} isVideo={isVideo} />
      <div className="one-docs-divider" />
      <DocsBody className="one-docs-body">
        <MDX
          components={
            { VideoEmbed } as Record<string, React.ComponentType<unknown>>
          }
        />
      </DocsBody>
    </DocsPage>
  );
}

export function generateStaticParams() {
  return tutorialsSource.generateParams("slug", "lang");
}

export async function generateMetadata(props: {
  params: Promise<{ lang: string; slug?: string[] }>;
}) {
  const params = await props.params;
  if (!isLocale(params.lang)) notFound();
  const page = tutorialsSource.getPage(params.slug, params.lang);
  if (!page) notFound();
  return createPageMetadata({
    title: page.data.title ?? "One CLI Tutorials",
    description: page.data.description,
    path: localizedTutorialsPath(params.lang, params.slug),
    locale: params.lang,
    alternates: alternateTutorialsLanguages(params.slug),
    type: "article",
  });
}

function ArticleBreadcrumb({
  lang,
  slug,
  title,
}: {
  lang: Locale;
  slug?: string[];
  title: string;
}) {
  const labels = uiText[lang];
  const isVideo = slug?.[0] === "videos";

  return (
    <nav className="one-docs-breadcrumb" aria-label="Breadcrumb">
      <a href={localizedDocsPath(lang)}>{labels.docs}</a>
      <ChevronRight className="size-3" />
      <a href={localizedTutorialsPath(lang)}>{labels.tutorials}</a>
      {isVideo ? (
        <>
          <ChevronRight className="size-3" />
          <span>{labels.videoSection}</span>
        </>
      ) : null}
      <ChevronRight className="size-3" />
      <span aria-current="page">{title}</span>
    </nav>
  );
}

function ArticleMeta({
  docPath,
  lang,
  isVideo,
}: {
  docPath: string;
  lang: Locale;
  isVideo: boolean;
}) {
  const labels = uiText[lang];

  return (
    <div className="one-docs-meta">
      <span>
        <Clock className="size-3.5" />
        {isVideo ? labels.videoMeta : labels.readTime}
      </span>
      <span>
        <GitCommit className="size-3.5" />
        {labels.updated}
      </span>
      <a
        href={`https://github.com/1cli-team/one-cli/blob/master/apps/docs/content/tutorials/${docPath}`}
        target="_blank"
        rel="noreferrer"
      >
        <FileText className="size-3.5" />
        {labels.editGithub}
      </a>
    </div>
  );
}

function getContentPath(lang: Locale, slug?: string[]) {
  if (!slug || slug.length === 0) {
    return `${lang}/index.md`;
  }
  const path = `${slug.join("/")}.md`;
  return `${lang}/${path}`;
}

function tocTitleToText(node: ReactNode): string {
  if (typeof node === "string" || typeof node === "number") {
    return String(node);
  }

  if (Array.isArray(node)) {
    return node.map(tocTitleToText).join("");
  }

  if (isValidElement<{ children?: ReactNode }>(node)) {
    return tocTitleToText(node.props.children);
  }

  return "";
}

const uiText: Record<
  Locale,
  {
    docs: string;
    tutorials: string;
    videoSection: string;
    readTime: string;
    videoMeta: string;
    updated: string;
    editGithub: string;
  }
> = {
  zh: {
    docs: "文档",
    tutorials: "教程",
    videoSection: "视频",
    readTime: "约 6 分钟",
    videoMeta: "视频 + 图文",
    updated: "3 天前更新",
    editGithub: "在 GitHub 编辑",
  },
  en: {
    docs: "Docs",
    tutorials: "Tutorials",
    videoSection: "Videos",
    readTime: "6 min read",
    videoMeta: "Video + walkthrough",
    updated: "Updated 3 days ago",
    editGithub: "Edit on GitHub",
  },
};
