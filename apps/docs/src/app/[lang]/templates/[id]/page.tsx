import { notFound, permanentRedirect } from "next/navigation";
import { isLocale, locales } from "@/i18n";
import { getTemplateById, templates } from "@/data/templates";
import { TemplateDetail } from "@/components/template-detail";
import { breadcrumbJsonLd, createPageMetadata, jsonLdScriptProps } from "@/lib/seo";

const retiredExamples = new Set([
  "mobile-starter", "desktop-starter", "landing-starter",
  "docs-starter", "consumer-starter", "admin-starter",
]);

type PageProps = { params: Promise<{ lang: string; id: string }> };

function resolveTemplate(lang: string, id: string) {
  if (!isLocale(lang)) notFound();
  if (retiredExamples.has(id)) permanentRedirect(`/${lang}/templates/`);
  const template = getTemplateById(id);
  if (!template) notFound();
  return { lang, template };
}

export function generateStaticParams() {
  return locales.flatMap((lang) => templates.map((template) => ({ lang, id: template.id })));
}

export async function generateMetadata({ params }: PageProps) {
  const input = await params;
  const { lang, template } = resolveTemplate(input.lang, input.id);
  return createPageMetadata({
    title: `${template.title[lang]} | One CLI`,
    description: template.tagline[lang],
    path: `/${lang}/templates/${template.id}/`,
    locale: lang,
    images: [template.cover],
    alternates: {
      "zh-Hans": `/zh/templates/${template.id}/`,
      en: `/en/templates/${template.id}/`,
      "x-default": `/zh/templates/${template.id}/`,
    },
  });
}

export default async function TemplatePage({ params }: PageProps) {
  const input = await params;
  const { lang, template } = resolveTemplate(input.lang, input.id);
  return (
    <>
      <script {...jsonLdScriptProps(breadcrumbJsonLd([
        { name: lang === "zh" ? "模板" : "Templates", path: `/${lang}/templates/` },
        { name: template.title[lang], path: `/${lang}/templates/${template.id}/` },
      ]))} />
      <TemplateDetail template={template} lang={lang} />
    </>
  );
}
