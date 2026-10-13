import { notFound } from "next/navigation";
import { isLocale } from "@/i18n";
import { generateHomeMetadata, LocalizedHomePage } from "../(home)/home-page";

export async function generateMetadata({ params }: { params: Promise<{ lang: string }> }) {
	const { lang } = await params;
	if (!isLocale(lang)) notFound();
	return generateHomeMetadata(lang);
}

export default async function Page({ params }: { params: Promise<{ lang: string }> }) {
	const { lang } = await params;
	if (!isLocale(lang)) notFound();
	return <LocalizedHomePage lang={lang} />;
}
