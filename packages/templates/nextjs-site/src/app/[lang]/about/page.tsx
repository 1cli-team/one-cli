import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { isLocale, messages } from "@/lib/i18n";

export async function generateMetadata({
	params,
}: {
	params: Promise<{ lang: string }>;
}): Promise<Metadata> {
	const { lang } = await params;
	if (!isLocale(lang)) notFound();
	return {
		title: messages[lang].about,
		description: messages[lang].aboutDescription,
		alternates: {
			canonical: `/${lang}/about/`,
			languages: { "en-US": "/en/about/", "zh-CN": "/zh/about/" },
		},
	};
}
export default async function Page({ params }: { params: Promise<{ lang: string }> }) {
	const { lang } = await params;
	if (!isLocale(lang)) notFound();
	return (
		<main className="mx-auto max-w-5xl space-y-6 px-6 py-24">
			<h1 className="text-4xl font-semibold">{messages[lang].aboutTitle}</h1>
			<p className="max-w-2xl text-lg text-muted-foreground">{messages[lang].aboutDescription}</p>
		</main>
	);
}
