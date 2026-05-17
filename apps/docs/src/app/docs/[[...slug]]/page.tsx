import { defaultLocale, localizedDocsPath } from "@/i18n";
import { source } from "@/lib/source";

export function generateStaticParams() {
  return source.getPages(defaultLocale).map((page) => ({
    slug: page.slugs,
  }));
}

export async function generateMetadata(props: {
  params: Promise<{ slug?: string[] }>;
}) {
  const { slug } = await props.params;
  const target = localizedDocsPath(defaultLocale, slug);

  return {
    title: "One CLI Docs",
    robots: {
      index: false,
      follow: true,
    },
    alternates: {
      canonical: target,
    },
  };
}

export default async function LegacyDocsPage(props: {
  params: Promise<{ slug?: string[] }>;
}) {
  const { slug } = await props.params;
  const target = localizedDocsPath(defaultLocale, slug);

  return (
    <main className="one-docs-legacy-redirect">
      <meta httpEquiv="refresh" content={`0; url=${target}`} />
      <h1>One CLI Docs moved</h1>
      <p>
        This page now lives at <a href={target}>{target}</a>.
      </p>
    </main>
  );
}
