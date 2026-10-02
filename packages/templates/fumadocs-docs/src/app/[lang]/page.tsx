import Link from "next/link";
import { notFound } from "next/navigation";
import { messages, isLocale } from "@/lib/i18n";
export default async function Page({ params }: { params: Promise<{ lang: string }> }) {
	const { lang } = await params;
	if (!isLocale(lang)) notFound();
	const text = messages[lang];
	return (
		<main className="mx-auto flex min-h-screen max-w-2xl flex-col items-center justify-center gap-6 px-6 text-center">
			<h1 className="text-4xl font-semibold">{text.title}</h1>
			<p className="text-muted-foreground">{text.description}</p>
			<Link
				className="rounded-lg bg-primary px-4 py-2 text-primary-foreground"
				href={`/${lang}/docs/`}
			>
				{text.read}
			</Link>
		</main>
	);
}
