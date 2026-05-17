import { DocsLayout } from "fumadocs-ui/layouts/docs";
import type { ReactNode } from "react";
import { SiteTopNav } from "@/components/site-top-nav";
import { isLocale } from "@/i18n";
import { source } from "@/lib/source";
import { CodeCopyEnhancer } from "../../docs/code-copy-enhancer";
import { DocsSidebar } from "../../docs/docs-sidebar";
import { baseOptions } from "../../layout.config";
import { notFound } from "next/navigation";

export default async function Layout({
  children,
  params,
}: {
  children: ReactNode;
  params: Promise<{ lang: string }>;
}) {
  const { lang: rawLang } = await params;
  if (!isLocale(rawLang)) notFound();
  const lang = rawLang;

  return (
    <>
      <SiteTopNav lang={lang} active="docs" />
      <CodeCopyEnhancer />
      <DocsLayout
        tree={source.getPageTree(lang)}
        {...baseOptions}
        containerProps={{
          className:
            "one-docs-layout md:[--fd-nav-height:64px] md:[--fd-sidebar-width:280px] xl:[--fd-toc-width:260px] xl:[--fd-page-width:1160px]",
        }}
        sidebar={{
          component: <DocsSidebar lang={lang} />,
          tabs: false,
        }}
        searchToggle={{ enabled: false }}
      >
        {children}
      </DocsLayout>
    </>
  );
}
