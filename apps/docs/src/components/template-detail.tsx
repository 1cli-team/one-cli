import { ArrowLeft, ArrowUpRight, Check } from "lucide-react";
import { Link } from "next-view-transitions";
import type { TemplateMeta } from "@/data/templates";
import type { Locale } from "@/i18n";
import { TemplateCoverImage } from "@/components/template-cover-image";
import { TemplateExamplesNav } from "@/components/template-examples-nav";
import { templateKindLabels } from "@/components/template-card";

const copy = {
  zh: {
    breadcrumb: "模板", overview: "适用场景", includes: "模板包含",
    id: "模板 ID", directory: "默认项目目录", toolchain: "工具链", none: "自行选择",
    guide: "查看模板使用指南", source: "查看模板源码", illustration: "模板概念插画",
  },
  en: {
    breadcrumb: "Templates", overview: "When to use it", includes: "What’s included",
    id: "Template ID", directory: "Default project directory", toolchain: "Toolchain", none: "Your choice",
    guide: "Read the template guide", source: "View template source", illustration: "Template concept illustration",
  },
};

export function TemplateDetail({ template, lang }: {
  template: TemplateMeta;
  lang: Locale;
}) {
  const text = copy[lang];
  const facts = [
    [text.id, template.id],
    [text.directory, `${template.directory}/`],
    [text.toolchain, template.toolchain === "none" ? text.none : template.toolchain === "go" ? "Go" : "Node.js"],
  ];

  return (
    <main className="min-h-screen bg-[#fafaf9] text-[#0a0a0a]">
      <TemplateExamplesNav lang={lang} templateId={template.id} />
      <section className="mx-auto flex w-full max-w-[1280px] flex-col gap-16 px-5 pb-28 pt-8 lg:px-16 lg:py-14">
        <nav aria-label={text.breadcrumb} className="flex min-w-0 items-center gap-2 text-sm text-stone-500">
          <Link href={`/${lang}/templates/`} className="inline-flex shrink-0 items-center gap-1 hover:text-[#0a0a0a]">
            <ArrowLeft className="size-3.5" aria-hidden="true" />{text.breadcrumb}
          </Link>
          <span aria-hidden="true" className="text-stone-300">/</span>
          <span aria-current="page" className="truncate text-stone-700">{template.title[lang]}</span>
        </nav>

        <header className="flex flex-col items-center gap-6 text-center">
          <div className="inline-flex items-center gap-2 rounded-full border border-stone-200 bg-white px-3 py-1 text-xs font-medium text-stone-700">
            <span className="size-1.5 rounded-full bg-[#ea580c]" />
            {templateKindLabels[template.kind][lang]}
          </div>
          <h1 className="text-4xl font-bold leading-tight text-[#0a0a0a] md:text-6xl">{template.title[lang]}</h1>
          <p className="max-w-[680px] text-base leading-7 text-stone-600 md:text-lg">{template.tagline[lang]}</p>
        </header>

        <div style={{ viewTransitionName: `example-card-${template.id}` }} className="relative aspect-[16/10] overflow-hidden rounded-2xl border border-stone-200 bg-stone-100 shadow-[0_1px_2px_rgba(10,10,10,0.04)]">
          <TemplateCoverImage src={template.cover} alt={text.illustration} className="object-cover" priority sizes="(min-width: 1280px) 1152px, calc(100vw - 40px)" />
        </div>

        <div className="grid gap-10 lg:grid-cols-[minmax(0,1fr)_320px] lg:gap-16">
          <div className="flex flex-col gap-8">
            <section className="flex flex-col gap-4">
              <h2 className="text-2xl font-bold text-stone-900">{text.overview}</h2>
              <p className="text-base leading-7 text-stone-600">{template.overview[lang]}</p>
            </section>
            <section className="flex flex-col gap-4">
              <h2 className="text-2xl font-bold text-stone-900">{text.includes}</h2>
              <ul className="flex flex-col gap-3">
                {template.features.map((feature, index) => (
                  <li key={index} className="flex items-start gap-3 text-sm leading-6 text-stone-600">
                    <Check className="mt-1 size-4 shrink-0 text-[#ea580c]" aria-hidden="true" />
                    {feature[lang]}
                  </li>
                ))}
              </ul>
            </section>
          </div>
          <aside className="flex flex-col gap-6 self-start rounded-xl border border-stone-200 bg-white p-6">
            <dl className="flex flex-col gap-5">
              {facts.map(([label, value]) => (
                <div key={label} className="flex flex-col gap-1.5">
                  <dt className="text-xs text-stone-500">{label}</dt>
                  <dd className="break-words font-mono text-sm text-stone-900">{value}</dd>
                </div>
              ))}
            </dl>
            <div className="flex flex-wrap gap-1.5">
              {template.tags.map((tag) => <span key={tag} className="rounded-full bg-stone-100 px-2.5 py-1 text-xs text-stone-600">{tag}</span>)}
            </div>
            <div className="flex flex-col items-start gap-3 border-t border-stone-100 pt-5 text-sm font-medium text-[#c2410c]">
              <Link href={`/${lang}/docs/templates/`} className="hover:underline">{text.guide}</Link>
              <a href={`https://github.com/1cli-team/one-cli/tree/master/packages/templates/${template.id}`} target="_blank" rel="noreferrer" className="inline-flex items-center gap-1 hover:underline">
                {text.source}<ArrowUpRight className="size-3.5" aria-hidden="true" />
              </a>
            </div>
          </aside>
        </div>
      </section>
    </main>
  );
}
