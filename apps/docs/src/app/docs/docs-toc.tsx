"use client";

import { TOCItem, type TOCItemType, useActiveAnchor } from "fumadocs-core/toc";
import { MessageSquare, Pencil } from "lucide-react";
import { PageTOC } from "fumadocs-ui/layouts/docs/page";
import type { Locale } from "@/i18n";

type DocsTocProps = {
  docPath: string;
  items: TOCItemType[];
  lang: Locale;
};

export function DocsToc({ docPath, items, lang }: DocsTocProps) {
  const activeAnchor = useActiveAnchor();
  const fallbackActive = items[0]?.url;
  const labels = tocLabels[lang];

  return (
    <PageTOC className="one-docs-toc">
      <p className="one-docs-toc-heading">{labels.heading}</p>
      <div className="one-docs-toc-list">
        {items.map((item) => {
          const active =
            item.url === `#${activeAnchor}` ||
            (activeAnchor === undefined && item.url === fallbackActive);

          return (
            <TOCItem
              className="one-docs-toc-link"
              data-active={active}
              data-depth={item.depth}
              href={item.url}
              key={item.url}
            >
              {item.title}
            </TOCItem>
          );
        })}
      </div>

      <div className="one-docs-toc-rule" />

      <div className="one-docs-toc-actions">
        <a
          href={`https://github.com/1cli-team/one-cli/blob/master/apps/docs/content/docs/${docPath}`}
          target="_blank"
          rel="noreferrer"
        >
          <Pencil className="size-[13px]" />
          {labels.edit}
        </a>
        <a
          href="https://github.com/1cli-team/one-cli/issues"
          target="_blank"
          rel="noreferrer"
        >
          <MessageSquare className="size-[13px]" />
          {labels.issue}
        </a>
      </div>
    </PageTOC>
  );
}

const tocLabels: Record<
  Locale,
  {
    heading: string;
    edit: string;
    issue: string;
  }
> = {
  zh: {
    heading: "本页目录",
    edit: "编辑本页",
    issue: "反馈问题",
  },
  en: {
    heading: "On this page",
    edit: "Edit this page",
    issue: "Report an issue",
  },
};
