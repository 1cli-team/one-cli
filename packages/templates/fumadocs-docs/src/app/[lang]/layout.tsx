import type { ReactNode } from "react";
import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { SiteDocument } from "@/components/site-document";
import { locales, isLocale, messages } from "@/lib/i18n";
export const dynamicParams = false;
export function generateStaticParams() {
	return locales.map((lang) => ({ lang }));
}
export async function generateMetadata({
	params,
}: {
	params: Promise<{ lang: string }>;
}): Promise<Metadata> {
	const { lang } = await params;
	if (!isLocale(lang)) notFound();
	return {
		title: { default: messages[lang].title, template: `%s | ${messages[lang].title}` },
		description: messages[lang].description,
	};
}
export default async function Layout({
	children,
	params,
}: {
	children: ReactNode;
	params: Promise<{ lang: string }>;
}) {
	const { lang } = await params;
	if (!isLocale(lang)) notFound();
	return <SiteDocument locale={lang}>{children}</SiteDocument>;
}
