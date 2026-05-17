"use client";

import {
  Copy,
  FileText,
  Globe2,
  LayoutDashboard,
  Layers3,
  Monitor,
  ShoppingBag,
  Smartphone,
  Sparkles,
  Wand2,
} from "lucide-react";
import { useMemo, useState } from "react";
import { examples, type Example, type ExampleCategory } from "@/data/examples";
import { TemplateExamplesNav } from "@/components/template-examples-nav";
import { CustomTemplateModal } from "@/components/custom-template-modal";
import { PresetCreateCommandDialog } from "@/components/preset-create-command-dialog";
import { TemplateCoverImage } from "@/components/template-cover-image";
import {
  shouldInterceptClick,
  useViewTransitionNavigate,
} from "@/lib/use-view-transition";

type ListLocale = "zh" | "en";

type ListCategory = ExampleCategory | "all";

const categoryOrder: ListCategory[] = [
  "all",
  "mobile",
  "desktop",
  "web",
  "consumer",
  "admin",
  "docs",
];

const categoryIcons: Record<ListCategory, typeof Layers3> = {
  all: Layers3,
  mobile: Smartphone,
  desktop: Monitor,
  web: Globe2,
  consumer: ShoppingBag,
  admin: LayoutDashboard,
  docs: FileText,
};

const copyText = {
  zh: {
    eyebrow: "TEMPLATE EXAMPLES",
    title: "从一个完整的起步套件开始。",
    body: "每个示例都是一个完整的 One CLI workspace 配置（项目 + 部署 + env）。直接复制创建命令、或点开查看默认页面与可粘贴 prompt。",
    customizeBtn: "自定义模板",
    filterAll: "全部",
    countSuffix: "个示例",
    copyCmd: "复制创建命令",
    copied: "已复制",
    categories: {
      all: "全部",
      mobile: "移动",
      desktop: "桌面",
      web: "Web",
      consumer: "C 端",
      admin: "后台",
      docs: "文档站",
    } satisfies Record<ListCategory, string>,
    notFoundTitle: "找不到完全匹配的示例？",
    notFoundBody:
      "用“自定义模板”自由组合项目、部署目标和 env，命令实时生成。",
  },
  en: {
    eyebrow: "TEMPLATE EXAMPLES",
    title: "Start from a complete starter kit.",
    body: "Each example is a full One CLI workspace (projects + deploy + env). Copy the create command directly, or open it to see the default pages and a paste-ready prompt.",
    customizeBtn: "Build your own",
    filterAll: "All",
    countSuffix: "examples",
    copyCmd: "Copy create command",
    copied: "Copied",
    categories: {
      all: "All",
      mobile: "Mobile",
      desktop: "Desktop",
      web: "Web",
      consumer: "Consumer",
      admin: "Admin",
      docs: "Docs",
    } satisfies Record<ListCategory, string>,
    notFoundTitle: "Don't see a perfect match?",
    notFoundBody:
      "Use “Build your own” to compose projects, deploy targets and env. Commands update live.",
  },
} satisfies Record<ListLocale, unknown>;

export function TemplateExamplesList({ lang }: { lang: ListLocale }) {
  const text = copyText[lang];
  const [activeCategory, setActiveCategory] = useState<ListCategory>("all");
  const [modalOpen, setModalOpen] = useState(false);
  const [copyExample, setCopyExample] = useState<Example | null>(null);

  const visible = useMemo<Example[]>(() => {
    if (activeCategory === "all") return examples;
    return examples.filter((e) => e.category === activeCategory);
  }, [activeCategory]);

  const counts = useMemo(() => {
    const map = new Map<ListCategory, number>();
    map.set("all", examples.length);
    for (const e of examples) {
      map.set(e.category, (map.get(e.category) ?? 0) + 1);
    }
    return map;
  }, []);

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
          <div className="flex flex-col items-start gap-2">
            <button
              type="button"
              onClick={() => setModalOpen(true)}
              className="inline-flex h-10 items-center gap-2 rounded-md bg-[#ea580c] px-4 text-sm font-semibold text-white shadow-[0_1px_2px_rgba(10,10,10,0.06)] hover:bg-[#c2410c]"
            >
              <Wand2 className="size-4" />
              {text.customizeBtn}
            </button>
          </div>
        </header>

        <div className="flex flex-wrap items-center gap-2">
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
                className={[
                  "inline-flex h-9 items-center gap-2 rounded-full border px-3.5 text-sm transition",
                  active
                    ? "border-orange-200 bg-orange-50 text-[#0a0a0a]"
                    : "border-stone-200 bg-white text-stone-600 hover:border-stone-300 hover:text-[#0a0a0a]",
                ].join(" ")}
              >
                <Icon className="size-4" />
                <span>{text.categories[category]}</span>
                <span className="font-mono text-xs text-stone-500">
                  {count}
                </span>
              </button>
            );
          })}
        </div>

        <div className="grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
          {visible.map((example) => (
            <ExampleCard
              key={example.id}
              example={example}
              lang={lang}
              text={text}
              onCopy={() => setCopyExample(example)}
            />
          ))}
        </div>

        <footer className="rounded-xl border border-dashed border-stone-300 bg-white p-8 text-center">
          <div className="mx-auto flex max-w-xl flex-col items-center gap-3">
            <Sparkles className="size-6 text-[#ea580c]" />
            <p className="text-base font-semibold text-stone-900">
              {text.notFoundTitle}
            </p>
            <p className="text-sm text-stone-600">{text.notFoundBody}</p>
            <button
              type="button"
              onClick={() => setModalOpen(true)}
              className="mt-2 inline-flex h-9 items-center gap-2 rounded-md border border-stone-300 bg-white px-4 text-sm font-semibold text-stone-900 hover:border-stone-400"
            >
              <Wand2 className="size-4" />
              {text.customizeBtn}
            </button>
          </div>
        </footer>
      </section>

      <CustomTemplateModal
        lang={lang}
        open={modalOpen}
        onClose={() => setModalOpen(false)}
      />
      <PresetCreateCommandDialog
        lang={lang}
        example={copyExample}
        open={copyExample !== null}
        onClose={() => setCopyExample(null)}
      />
    </main>
  );
}

function ExampleCard({
  example,
  lang,
  text,
  onCopy,
}: {
  example: Example;
  lang: ListLocale;
  text: (typeof copyText)[ListLocale];
  onCopy: () => void;
}) {
  const Icon = categoryIcons[example.category];
  const navigate = useViewTransitionNavigate();
  const href = `/${lang}/templates/${example.id}/`;
  const cardTransitionName = `example-card-${example.id}`;

  function handleCopy(e: React.MouseEvent) {
    e.preventDefault();
    e.stopPropagation();
    onCopy();
  }

  function handleClick(e: React.MouseEvent<HTMLAnchorElement>) {
    if (!shouldInterceptClick(e)) return;
    e.preventDefault();
    navigate(href);
  }

  return (
    <a
      href={href}
      onClick={handleClick}
      className="group flex flex-col overflow-hidden rounded-xl border border-stone-200 bg-white shadow-[0_1px_2px_rgba(10,10,10,0.04)] transition hover:-translate-y-0.5 hover:border-stone-300 hover:shadow-[0_4px_16px_rgba(10,10,10,0.06)]"
      style={{ viewTransitionName: cardTransitionName }}
    >
      <div className="relative aspect-[16/10] w-full overflow-hidden bg-stone-100">
        <TemplateCoverImage
          src={example.cover}
          alt=""
          className="object-cover transition group-hover:scale-[1.02]"
          sizes="(min-width: 1024px) 370px, (min-width: 640px) 50vw, 100vw"
        />
        <div className="absolute left-3 top-3 inline-flex items-center gap-1.5 rounded-full bg-white/90 px-2.5 py-1 text-xs font-medium text-stone-700 shadow-[0_1px_2px_rgba(10,10,10,0.06)] backdrop-blur">
          <Icon className="size-3.5" />
          {text.categories[example.category]}
        </div>
        <button
          type="button"
          onClick={handleCopy}
          aria-label={text.copyCmd}
          className="absolute right-3 top-3 inline-flex size-8 items-center justify-center rounded-full bg-white/90 text-stone-700 shadow-[0_1px_2px_rgba(10,10,10,0.06)] backdrop-blur transition hover:bg-white hover:text-[#0a0a0a]"
        >
          <Copy className="size-4" />
        </button>
      </div>
      <div className="flex flex-1 flex-col gap-3 p-5">
        <h3 className="text-lg font-semibold text-stone-900">
          {example.title[lang]}
        </h3>
        <p className="text-sm leading-6 text-stone-600">
          {example.tagline[lang]}
        </p>
      </div>
    </a>
  );
}
