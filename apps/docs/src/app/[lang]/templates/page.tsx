import { isLocale, type Locale } from "@/i18n";
import { TemplateExamplesList } from "@/components/template-examples-list";
import { notFound } from "next/navigation";
import { templates } from "@/data/templates";
import { createPageMetadata, itemListJsonLd, jsonLdScriptProps } from "@/lib/seo";

export function generateStaticParams() {
  return [{ lang: "zh" }, { lang: "en" }];
}

export async function generateMetadata(props: { params: Promise<{ lang: string }> }) {
  const { lang } = await props.params;
  if (!isLocale(lang)) notFound();
  return createPageMetadata({
    title: lang === "zh" ? "模板目录 | One CLI" : "Template Catalog | One CLI",
    description: lang === "zh" ? "浏览 One CLI 内置模板的技术栈、用途和项目目录。" : "Browse the stacks, purpose, and project directories of One CLI’s built-in templates.",
    path: localizedTemplatePath(lang),
    locale: lang,
    alternates: { "zh-Hans": localizedTemplatePath("zh"), en: localizedTemplatePath("en"), "x-default": localizedTemplatePath("zh") },
  });
}

export default async function TemplatesPage(props: { params: Promise<{ lang: string }> }) {
  const { lang } = await props.params;
  if (!isLocale(lang)) notFound();
  return (
    <>
      <script {...jsonLdScriptProps(itemListJsonLd({
        name: lang === "zh" ? "One CLI 模板目录" : "One CLI Template Catalog",
        description: lang === "zh" ? "One CLI 内置项目模板" : "Built-in One CLI project templates",
        items: templates.map((template) => ({ name: template.title[lang], description: template.tagline[lang], path: `${localizedTemplatePath(lang)}${template.id}/` })),
      }))} />
      <TemplateExamplesList lang={lang} templates={templates} />
    </>
  );
}

function localizedTemplatePath(lang: Locale) {
  return `/${lang}/templates/`;
}
