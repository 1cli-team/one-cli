import type { ReactNode } from "react";
import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { SiteDocument } from "@/components/site-document";
import { ThemeToggle } from "@/components/theme-toggle";
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
		metadataBase: new URL(process.env.NEXT_PUBLIC_SITE_URL ?? "https://example.com"),
		title: { default: messages[lang].name, template: `%s | ${messages[lang].name}` },
		description: messages[lang].description,
	};
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
	const text = messages[lang];
	return (
		<SiteDocument locale={lang}>
			<header className="mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-4 border-b px-6 py-5">
				<Link href={`/${lang}/`} className="font-semibold">
					{text.name}
				</Link>
				<nav aria-label={text.home} className="flex flex-wrap items-center gap-4 text-sm">
					<Link href={`/${lang}/`}>{text.home}</Link>
					<Link href={`/${lang}/about/`}>{text.about}</Link>
					<Link href={lang === "en" ? "/zh/" : "/en/"}>{lang === "en" ? "中文" : "English"}</Link>
					<ThemeToggle locale={lang} />
				</nav>
			</header>
			{children}
		</SiteDocument>
	);
}
