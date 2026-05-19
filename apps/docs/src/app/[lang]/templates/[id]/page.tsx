import { isLocale, locales, type Locale } from "@/i18n";
import { TemplateExampleDetail } from "@/components/template-example-detail";
import { getExample, getExampleIds } from "@/lib/examples";
import { notFound } from "next/navigation";
import {
  breadcrumbJsonLd,
  createPageMetadata,
  itemListJsonLd,
  jsonLdScriptProps,
} from "@/lib/seo";

export function generateStaticParams() {
  const ids = getExampleIds();
  return locales.flatMap((lang) => ids.map((id) => ({ lang, id })));
}

export async function generateMetadata(props: {
  params: Promise<{ lang: string; id: string }>;
}) {
  const { lang: rawLang, id } = await props.params;
  if (!isLocale(rawLang)) notFound();
  const example = getExample(id);
  if (!example) notFound();
  const lang = rawLang;

  return createPageMetadata({
    title:
      lang === "zh"
        ? `${example.title.zh} | One CLI 模板示例`
        : `${example.title.en} | One CLI Template Examples`,
    description:
      lang === "zh" ? example.tagline.zh : example.tagline.en,
    path: detailPath(lang, id),
    locale: lang,
    alternates: {
      "zh-Hans": detailPath("zh", id),
      en: detailPath("en", id),
      "x-default": detailPath("zh", id),
    },
  });
}

export default async function TemplateExampleRoute(props: {
  params: Promise<{ lang: string; id: string }>;
}) {
  const { lang: rawLang, id } = await props.params;
  if (!isLocale(rawLang)) notFound();
  const example = getExample(id);
  if (!example) notFound();

  return (
    <>
      <script
        {...jsonLdScriptProps([
          itemListJsonLd({
            name: example.title[rawLang],
            description: example.tagline[rawLang],
            items: example.baseTemplates.map((template) => ({
              name: template,
              path: detailPath(rawLang, id),
            })),
          }),
          breadcrumbJsonLd([
            { name: "One CLI", path: `/${rawLang}/` },
            {
              name: rawLang === "zh" ? "模板示例" : "Template Examples",
              path: `/${rawLang}/templates/`,
            },
            { name: example.title[rawLang], path: detailPath(rawLang, id) },
          ]),
        ])}
      />
      <TemplateExampleDetail example={example} lang={rawLang} />
    </>
  );
}

function detailPath(lang: Locale, id: string) {
  return `/${lang}/templates/${id}/`;
}
