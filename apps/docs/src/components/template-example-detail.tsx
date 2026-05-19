"use client";

import {
  ArrowLeft,
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
import { Link } from "next-view-transitions";
import { useMemo, useState } from "react";
import { examples, type Example, type ExampleCategory } from "@/data/examples";
import { getTemplateById } from "@/data/templates";
import { TemplateExamplesNav } from "@/components/template-examples-nav";
import { CustomTemplateModal } from "@/components/custom-template-modal";
import { PresetCreateCommandDialog } from "@/components/preset-create-command-dialog";
import { TemplateCoverImage } from "@/components/template-cover-image";
import {
  shouldInterceptClick,
  useViewTransitionNavigate,
} from "@/lib/use-view-transition";

type DetailLocale = "zh" | "en";

const categoryIcons: Record<ExampleCategory, typeof Layers3> = {
  mobile: Smartphone,
  desktop: Monitor,
  web: Globe2,
  consumer: ShoppingBag,
  admin: LayoutDashboard,
  docs: FileText,
};

const categoryLabels: Record<ExampleCategory, { zh: string; en: string }> = {
  mobile: { zh: "移动", en: "Mobile" },
  desktop: { zh: "桌面", en: "Desktop" },
  web: { zh: "Web", en: "Web" },
  consumer: { zh: "C 端", en: "Consumer" },
  admin: { zh: "后台", en: "Admin" },
  docs: { zh: "文档", en: "Docs" },
};

const copy = {
  zh: {
    breadcrumb: "模板",
    copied: "已复制",
    copyCmd: "复制创建命令",
    related: "也可以看看",
    templates: "包含模板",
    customizeCard: {
      title: "想自由组合？",
      body: "打开自定义模板，挑选项目 / 部署 / env，命令实时生成。",
      cta: "自定义模板",
    },
  },
  en: {
    breadcrumb: "Templates",
    copied: "Copied",
    copyCmd: "Copy create command",
    related: "Other examples",
    templates: "Templates included",
    customizeCard: {
      title: "Want a custom mix?",
      body: "Open the builder to pick projects, deploy targets and env. Commands update live.",
      cta: "Build your own",
    },
  },
} satisfies Record<DetailLocale, unknown>;

export function TemplateExampleDetail({
  example,
  lang,
}: {
  example: Example;
  lang: DetailLocale;
}) {
  const text = copy[lang];
  const Icon = categoryIcons[example.category];
  const [copyDialogOpen, setCopyDialogOpen] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);

  const related = useMemo(
    () => examples.filter((e) => e.id !== example.id).slice(0, 3),
    [example.id],
  );

  const templateLabels = useMemo(
    () =>
      example.baseTemplates
        .map((id) => getTemplateById(id))
        .filter((t): t is NonNullable<typeof t> => t !== null),
    [example.baseTemplates],
  );

  return (
    <main className="min-h-screen bg-[#fafaf9] text-[#0a0a0a]">
      <TemplateExamplesNav lang={lang} />

      <section className="mx-auto flex w-full max-w-[1280px] flex-col gap-16 px-5 pb-28 pt-8 lg:px-16 lg:py-14">
        <nav className="flex items-center gap-2 text-sm text-stone-500">
          <Link
            href={`/${lang}/templates/`}
            className="inline-flex items-center gap-1 hover:text-[#0a0a0a]"
          >
            <ArrowLeft className="size-3.5" />
            {text.breadcrumb}
          </Link>
          <span className="text-stone-300">/</span>
          <span className="truncate text-stone-700">{example.title[lang]}</span>
        </nav>

        <header className="flex flex-col items-center gap-6 text-center">
          <div className="inline-flex items-center gap-2 rounded-full border border-stone-200 bg-white px-3 py-1 text-xs font-medium text-stone-700">
            <Icon className="size-3.5 text-[#ea580c]" />
            {categoryLabels[example.category][lang]}
          </div>
          <h1 className="text-4xl font-bold leading-tight text-[#0a0a0a] md:text-6xl">
            {example.title[lang]}
          </h1>
          <p className="max-w-[680px] text-base leading-7 text-stone-600 md:text-lg">
            {example.tagline[lang]}
          </p>
          <div className="flex flex-wrap items-center justify-center gap-3 pt-2">
            <button
              type="button"
              onClick={() => setCopyDialogOpen(true)}
              className="inline-flex h-11 items-center gap-2 rounded-md bg-[#ea580c] px-5 text-sm font-semibold text-white shadow-[0_1px_2px_rgba(10,10,10,0.06)] hover:bg-[#c2410c]"
            >
              <Copy className="size-4" />
              {text.copyCmd}
            </button>
          </div>
        </header>

        <div
          className="relative aspect-[16/10] overflow-hidden rounded-2xl border border-stone-200 bg-stone-100 shadow-[0_1px_2px_rgba(10,10,10,0.04)]"
          style={{ viewTransitionName: `example-card-${example.id}` }}
        >
          <TemplateCoverImage
            src={example.cover}
            alt=""
            className="object-cover"
            priority
            sizes="(min-width: 1280px) 1152px, calc(100vw - 40px)"
          />
        </div>

        {templateLabels.length > 0 && (
          <section className="flex flex-col gap-4">
            <h2 className="text-base font-semibold text-stone-900">
              {text.templates}
            </h2>
            <div className="flex flex-wrap gap-3">
              {templateLabels.map((t) => (
                <div
                  key={t.id}
                  className="flex flex-col gap-1 rounded-lg border border-stone-200 bg-white px-4 py-3"
                >
                  <span className="text-sm font-semibold text-stone-900">
                    {t.title[lang]}
                  </span>
                  <span className="text-xs text-stone-500">
                    {t.tagline[lang]}
                  </span>
                </div>
              ))}
            </div>
            <div className="flex flex-wrap items-center gap-1.5">
              {example.tags.map((tag) => (
                <span
                  key={tag}
                  className="inline-flex items-center rounded-full bg-stone-100 px-2.5 py-1 text-xs text-stone-600"
                >
                  {tag}
                </span>
              ))}
            </div>
          </section>
        )}

        <section className="flex flex-col gap-5">
          <h2 className="text-2xl font-bold text-stone-900">{text.related}</h2>
          <div className="grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
            {related.map((other) => (
              <RelatedCard key={other.id} example={other} lang={lang} />
            ))}
          </div>
        </section>

        <aside className="rounded-2xl border border-stone-200 bg-white p-6 shadow-[0_1px_2px_rgba(10,10,10,0.04)]">
          <div className="flex flex-col items-start gap-4 sm:flex-row sm:items-center sm:justify-between">
            <div className="flex items-start gap-3">
              <span className="inline-flex size-10 items-center justify-center rounded-md bg-orange-50 text-[#ea580c]">
                <Sparkles className="size-5" />
              </span>
              <div>
                <p className="text-base font-semibold text-stone-900">
                  {text.customizeCard.title}
                </p>
                <p className="mt-1 text-sm text-stone-600">
                  {text.customizeCard.body}
                </p>
              </div>
            </div>
            <button
              type="button"
              onClick={() => setModalOpen(true)}
              className="inline-flex h-10 items-center gap-2 rounded-md bg-[#ea580c] px-4 text-sm font-semibold text-white hover:bg-[#c2410c]"
            >
              <Wand2 className="size-4" />
              {text.customizeCard.cta}
            </button>
          </div>
        </aside>
      </section>

      <CustomTemplateModal
        lang={lang}
        open={modalOpen}
        onClose={() => setModalOpen(false)}
      />
      <PresetCreateCommandDialog
        lang={lang}
        example={example}
        open={copyDialogOpen}
        onClose={() => setCopyDialogOpen(false)}
      />
    </main>
  );
}

function RelatedCard({
  example,
  lang,
}: {
  example: Example;
  lang: DetailLocale;
}) {
  const Icon = categoryIcons[example.category];
  const navigate = useViewTransitionNavigate();
  const href = `/${lang}/templates/${example.id}/`;

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
      style={{ viewTransitionName: `example-card-${example.id}` }}
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
          {categoryLabels[example.category][lang]}
        </div>
      </div>
      <div className="flex flex-1 flex-col gap-1.5 p-4">
        <h3 className="text-base font-semibold text-stone-900">
          {example.title[lang]}
        </h3>
        <p className="line-clamp-2 text-xs leading-5 text-stone-600">
          {example.tagline[lang]}
        </p>
      </div>
    </a>
  );
}
