import { source } from "@/lib/source";
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
  alternateDocsLanguages,
  htmlLang,
  isLocale,
  localizedDocsPath,
  type Locale,
} from "@/i18n";
import { DocsToc } from "../../../docs/docs-toc";

export default async function Page(props: {
  params: Promise<{ lang: string; slug?: string[] }>;
}) {
  const params = await props.params;
  if (!isLocale(params.lang)) notFound();
  const lang = params.lang;
  const page = source.getPage(params.slug, lang);
  if (!page) notFound();

  // page.data carries fumadocs-mdx exports (body / toc / structuredData)
  // but the source-loader's PageData generic loses the extra fields, so
  // cast through a minimal shape rather than `any`.
  const data = page.data as unknown as {
    title: string;
    description?: string;
    body: React.ComponentType;
    toc?: import("fumadocs-core/server").TableOfContents;
    full?: boolean;
  };
  const MDX = data.body;
  const docPath = getContentPath(lang, params.slug);
  const toc = (data.toc ?? []).map((item) => ({
    ...item,
    title: tocTitleToText(item.title),
  }));

  return (
    <DocsPage
      toc={toc}
      full={data.full}
      breadcrumb={{
        component: (
          <ArticleBreadcrumb
            lang={lang}
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
      <DocsTitle className="one-docs-title">{data.title}</DocsTitle>
      <DocsDescription className="one-docs-description">
        {data.description}
      </DocsDescription>
      <ArticleMeta docPath={docPath} lang={lang} />
      <div className="one-docs-divider" />
      <DocsBody className="one-docs-body">
        <MDX />
      </DocsBody>
    </DocsPage>
  );
}

export function generateStaticParams() {
  return source.generateParams("slug", "lang");
}

export async function generateMetadata(props: {
  params: Promise<{ lang: string; slug?: string[] }>;
}) {
  const params = await props.params;
  if (!isLocale(params.lang)) notFound();
  const page = source.getPage(params.slug, params.lang);
  if (!page) notFound();
  return {
    title: page.data.title,
    description: page.data.description,
    alternates: {
      canonical: localizedDocsPath(params.lang, params.slug),
      languages: alternateDocsLanguages(params.slug),
    },
    other: {
      "content-language": htmlLang[params.lang],
    },
  };
}

function ArticleBreadcrumb({
  lang,
  title,
}: {
  lang: Locale;
  title: string;
}) {
  const labels = uiText[lang];

  return (
    <nav className="one-docs-breadcrumb" aria-label="Breadcrumb">
      <a href={localizedDocsPath(lang)}>{labels.docs}</a>
      <ChevronRight className="size-3" />
      <span aria-current="page">{title}</span>
    </nav>
  );
}

function ArticleMeta({ docPath, lang }: { docPath: string; lang: Locale }) {
  const labels = uiText[lang];

  return (
    <div className="one-docs-meta">
      <span>
        <Clock className="size-3.5" />
        {labels.readTime}
      </span>
      <span>
        <GitCommit className="size-3.5" />
        {labels.updated}
      </span>
      <a
        href={`https://github.com/torchstellar-team/one-cli/blob/master/apps/docs/content/docs/${docPath}`}
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
    return `${lang}/index.mdx`;
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
    readTime: string;
    updated: string;
    editGithub: string;
  }
> = {
  zh: {
    docs: "文档",
    readTime: "约 6 分钟",
    updated: "3 天前更新",
    editGithub: "在 GitHub 编辑",
  },
  en: {
    docs: "Docs",
    readTime: "6 min read",
    updated: "Updated 3 days ago",
    editGithub: "Edit on GitHub",
  },
};
