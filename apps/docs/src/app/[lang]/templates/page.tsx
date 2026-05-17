import { isLocale, type Locale } from "@/i18n";
import { TemplateExamplesList } from "@/components/template-examples-list";
import { notFound } from "next/navigation";
import { examples } from "@/data/examples";
import {
  createPageMetadata,
  itemListJsonLd,
  jsonLdScriptProps,
} from "@/lib/seo";

export function generateStaticParams() {
  return [{ lang: "zh" }, { lang: "en" }];
}

export async function generateMetadata(props: {
  params: Promise<{ lang: string }>;
}) {
  const { lang: rawLang } = await props.params;
  if (!isLocale(rawLang)) notFound();
  const lang = rawLang;

  return createPageMetadata({
    title:
      lang === "zh" ? "模板示例 | One CLI" : "Template Examples | One CLI",
    description:
      lang === "zh"
        ? "精选的 One CLI 模板示例：移动端、桌面端、Web、C 端、后台、文档站。一键复制 prompt，让 Claude 通过 one-cli skill 帮你搭起来。"
        : "Hand-picked One CLI examples: mobile, desktop, web, consumer, admin, docs. Copy the prompt and let Claude scaffold it via the one-cli skill.",
    path: localizedTemplatePath(lang),
    locale: lang,
    alternates: alternateTemplateLanguages(),
  });
}

export default async function TemplatesPage(props: {
  params: Promise<{ lang: string }>;
}) {
  const { lang: rawLang } = await props.params;
  if (!isLocale(rawLang)) notFound();

  return (
    <>
      <script
        {...jsonLdScriptProps(
          itemListJsonLd({
            name:
              rawLang === "zh" ? "One CLI 模板示例" : "One CLI Template Examples",
            description:
              rawLang === "zh"
                ? "One CLI 可组合模板示例"
                : "Composable One CLI template examples",
            items: examples.map((example) => ({
              name: example.title[rawLang],
              description: example.tagline[rawLang],
              path: `${localizedTemplatePath(rawLang)}${example.id}/`,
            })),
          }),
        )}
      />
      <TemplateExamplesList lang={rawLang} />
    </>
  );
}

function localizedTemplatePath(lang: Locale) {
  return `/${lang}/templates/`;
}

function alternateTemplateLanguages() {
  return {
    "zh-Hans": localizedTemplatePath("zh"),
    en: localizedTemplatePath("en"),
    "x-default": localizedTemplatePath("zh"),
  };
}
