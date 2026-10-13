import type { ReactNode } from "react";
import { notFound } from "next/navigation";
import { SiteDocument, siteMetadata } from "@/components/site-document";
import { locales, isLocale } from "@/i18n";

export const metadata = siteMetadata;
export const dynamicParams = false;

export function generateStaticParams() {
	return locales.map((lang) => ({ lang }));
}

export default async function Layout({
	params,
	children,
}: {
	params: Promise<{ lang: string }>;
	children: ReactNode;
}) {
	const { lang } = await params;
	if (!isLocale(lang)) notFound();
	return <SiteDocument locale={lang}>{children}</SiteDocument>;
}
