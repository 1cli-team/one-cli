import { notFound } from "next/navigation";
import { DocsPage, DocsBody, DocsTitle, DocsDescription } from "fumadocs-ui/page";
import { source } from "@/lib/source";
import { getMDXComponents } from "@/components/mdx";
import { isLocale } from "@/lib/i18n";

export const dynamicParams = false;
export function generateStaticParams() {
	return source.generateParams("slug", "lang");
}
export async function generateMetadata({
	params,
}: {
	params: Promise<{ lang: string; slug?: string[] }>;
}) {
	const { lang, slug } = await params;
	if (!isLocale(lang)) notFound();
	const page = source.getPage(slug, lang);
	if (!page) notFound();
	return { title: page.data.title, description: page.data.description };
}
export default async function Page({
	params,
}: {
	params: Promise<{ lang: string; slug?: string[] }>;
}) {
	const { lang, slug } = await params;
	if (!isLocale(lang)) notFound();
	const page = source.getPage(slug, lang);
	if (!page) notFound();
	const MDX = page.data.body;
	return (
		<DocsPage toc={page.data.toc} full={page.data.full}>
			<DocsTitle>{page.data.title}</DocsTitle>
			<DocsDescription>{page.data.description}</DocsDescription>
			<DocsBody>
				<MDX components={getMDXComponents()} />
			</DocsBody>
		</DocsPage>
	);
}
