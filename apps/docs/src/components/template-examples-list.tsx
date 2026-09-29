"use client";

import { Boxes, Globe2, Layers3, Server } from "lucide-react";
import { useMemo, useState } from "react";
import type { TemplateKind, TemplateMeta } from "@/data/templates";
import { TemplateExamplesNav } from "@/components/template-examples-nav";
import { TemplateCard } from "@/components/template-card";

type ListLocale = "zh" | "en";
type ListCategory = TemplateKind | "all";

const categoryOrder: ListCategory[] = ["all", "frontend", "backend", "library"];
const categoryIcons: Record<ListCategory, typeof Layers3> = {
  all: Layers3,
  frontend: Globe2,
  backend: Server,
  library: Boxes,
};

const copyText = {
  zh: {
    eyebrow: "模板目录",
    title: "从一个模板开始。",
    body: "浏览 One CLI 内置模板，了解技术栈与用途。选择后，在工作区通过 one add 添加项目。",
    filters: "按模板类型筛选",
    categories: {
      all: "全部",
      frontend: "前端应用",
      backend: "后端服务",
      library: "共享库",
    } satisfies Record<ListCategory, string>,
    count: (count: number) => `${count} 个模板`,
  },
  en: {
    eyebrow: "TEMPLATE CATALOG",
    title: "Start from a template.",
    body: "Browse One CLI’s built-in templates to compare their stacks and purpose. Use one add in your workspace to add the projects you need.",
    filters: "Filter by template type",
    categories: {
      all: "All",
      frontend: "Frontend apps",
      backend: "Backend services",
      library: "Shared libraries",
    } satisfies Record<ListCategory, string>,
    count: (count: number) => `${count} templates`,
  },
};

export function TemplateExamplesList({
  lang,
  templates,
}: {
  lang: ListLocale;
  templates: TemplateMeta[];
}) {
  const text = copyText[lang];
  const [activeCategory, setActiveCategory] = useState<ListCategory>("all");
  const visible = useMemo(() => {
    if (activeCategory === "all") return templates;
    return templates.filter((template) => template.kind === activeCategory);
  }, [activeCategory, templates]);
  const counts = useMemo(() => {
    const map = new Map<ListCategory, number>();
    map.set("all", templates.length);
    for (const template of templates) {
      map.set(template.kind, (map.get(template.kind) ?? 0) + 1);
    }
    return map;
  }, [templates]);

  return (
    <main className="min-h-screen bg-[#fafaf9] text-[#0a0a0a]">
      <TemplateExamplesNav lang={lang} />

      <section className="mx-auto flex w-full max-w-[1280px] flex-col gap-12 px-5 pb-28 pt-10 lg:px-16 lg:py-20">
        <header className="flex flex-col justify-between gap-8 lg:flex-row lg:items-end">
          <div className="max-w-[760px]">
            <p className="flex items-center gap-2 font-mono text-xs font-semibold text-[#ea580c]">
              <span className="size-1.5 rounded-full bg-[#ea580c]" />
              {text.eyebrow}
            </p>
            <h1 className="mt-4 text-4xl font-bold leading-[1.04] text-[#0a0a0a] md:text-6xl">
              {text.title}
            </h1>
            <p className="mt-5 max-w-[680px] text-base leading-7 text-stone-600">
              {text.body}
            </p>
          </div>
        </header>

        <div className="flex flex-wrap items-center gap-2" role="group" aria-label={text.filters}>
          {categoryOrder.map((category) => {
            const Icon = categoryIcons[category];
            const count = counts.get(category) ?? 0;
            const active = activeCategory === category;
            if (count === 0 && category !== "all") return null;
            return (
              <button
                key={category}
                type="button"
                onClick={() => setActiveCategory(category)}
                aria-pressed={active}
                className={[
                  "inline-flex h-9 items-center gap-2 rounded-full border px-3.5 text-sm transition",
                  active
                    ? "border-orange-200 bg-orange-50 text-[#0a0a0a]"
                    : "border-stone-200 bg-white text-stone-600 hover:border-stone-300 hover:text-[#0a0a0a]",
                ].join(" ")}
              >
                <Icon className="size-4" aria-hidden="true" />
                <span>{text.categories[category]}</span>
                <span className="font-mono text-xs text-stone-500">{count}</span>
              </button>
            );
          })}
        </div>

        <p className="sr-only" role="status" aria-live="polite">{text.count(visible.length)}</p>
        <div className="grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
          {visible.map((template) => (
            <TemplateCard key={template.id} template={template} lang={lang} />
          ))}
        </div>
      </section>
    </main>
  );
}
