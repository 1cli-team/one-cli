import type { ReactNode } from "react";
import { notFound } from "next/navigation";
import { DocsLayout } from "fumadocs-ui/layouts/docs";
import { source } from "@/lib/source";
import { isLocale, messages, i18n } from "@/lib/i18n";
export default async function Layout({
	children,
	params,
}: {
	children: ReactNode;
	params: Promise<{ lang: string }>;
}) {
	const { lang } = await params;
	if (!isLocale(lang)) notFound();
	return (
		<DocsLayout
			tree={source.getPageTree(lang)}
			i18n={{
				languages: i18n.languages,
				defaultLanguage: i18n.defaultLanguage,
				hideLocale: i18n.hideLocale,
			}}
			nav={{ title: messages[lang].title, url: `/${lang}/` }}
		>
			{children}
		</DocsLayout>
	);
}
