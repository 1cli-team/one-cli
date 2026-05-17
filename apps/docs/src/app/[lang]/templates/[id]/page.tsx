import { htmlLang, isLocale, locales, type Locale } from "@/i18n";
import { TemplateExampleDetail } from "@/components/template-example-detail";
import { getExample, getExampleIds } from "@/lib/examples";
import { notFound } from "next/navigation";

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

  return {
    title:
      lang === "zh"
        ? `${example.title.zh} | One CLI 模板示例`
        : `${example.title.en} | One CLI Template Examples`,
    description:
      lang === "zh" ? example.tagline.zh : example.tagline.en,
    alternates: {
      canonical: detailPath(lang, id),
      languages: {
        "zh-Hans": detailPath("zh", id),
        en: detailPath("en", id),
        "x-default": detailPath("zh", id),
      },
    },
    other: {
      "content-language": htmlLang[lang],
    },
  };
}

export default async function TemplateExampleRoute(props: {
  params: Promise<{ lang: string; id: string }>;
}) {
  const { lang: rawLang, id } = await props.params;
  if (!isLocale(rawLang)) notFound();
  const example = getExample(id);
  if (!example) notFound();

  return <TemplateExampleDetail example={example} lang={rawLang} />;
}

function detailPath(lang: Locale, id: string) {
  return `/${lang}/templates/${id}/`;
}
