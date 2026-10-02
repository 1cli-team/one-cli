import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { ArrowRight } from "lucide-react";
import { isLocale, messages } from "@/lib/i18n";

export async function generateMetadata({
	params,
}: {
	params: Promise<{ lang: string }>;
}): Promise<Metadata> {
	const { lang } = await params;
	if (!isLocale(lang)) notFound();
	return {
		title: messages[lang].title,
		description: messages[lang].description,
		alternates: {
			canonical: `/${lang}/`,
			languages: { "en-US": "/en/", "zh-CN": "/zh/" },
		},
	};
}

export default async function Page({ params }: { params: Promise<{ lang: string }> }) {
	const { lang } = await params;
	if (!isLocale(lang)) notFound();
	const text = messages[lang];
	return (
		<main className="mx-auto max-w-5xl px-6 py-24">
			<section className="max-w-2xl space-y-6">
				<h1 className="text-4xl font-semibold tracking-tight sm:text-6xl">{text.title}</h1>
				<p className="text-lg text-muted-foreground">{text.description}</p>
				<div className="flex flex-wrap items-center gap-4">
					<Link
						href={`/${lang}/about/`}
						className="inline-flex items-center gap-2 rounded-lg bg-primary px-4 py-2 text-primary-foreground"
					>
						{text.start}
						<ArrowRight className="size-4" />
					</Link>
				</div>
			</section>
		</main>
	);
}
