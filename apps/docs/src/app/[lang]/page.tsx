import { isLocale, type Locale } from "@/i18n";
import {
  generateHomeMetadata,
  LocalizedHomePage,
} from "../(home)/home-page";
import { notFound } from "next/navigation";

export function generateStaticParams() {
  return [{ lang: "zh" }, { lang: "en" }];
}

export async function generateMetadata(props: {
  params: Promise<{ lang: string }>;
}) {
  const { lang: rawLang } = await props.params;
  if (!isLocale(rawLang)) notFound();

  return generateHomeMetadata(rawLang);
}

export default async function LocalizedHomeRoute(props: {
  params: Promise<{ lang: string }>;
}) {
  const { lang: rawLang } = await props.params;
  if (!isLocale(rawLang)) notFound();

  return <LocalizedHomePage lang={rawLang as Locale} />;
}
