import type { Metadata } from "next";
import { htmlLang, type Locale } from "@/i18n";

export const siteUrl = "https://1cli.dev";
export const siteName = "One CLI";
export const defaultDescription =
  "One CLI is a scaffolding and governance tool for AI-native monorepo workspaces, templates, manifests, local configuration, and agent-ready command flows.";

type PageMetadataInput = {
  title: string;
  description?: string;
  path: string;
  locale?: Locale;
  alternates?: Record<string, string>;
  type?: "website" | "article";
  images?: string[];
};

type ArticleJsonLdInput = {
  title: string;
  description?: string;
  path: string;
  locale: Locale;
  datePublished?: string;
  dateModified?: string;
  author?: string;
  tags?: string[];
  section?: string;
};

type ItemListInput = {
  name: string;
  description?: string;
  items: Array<{
    name: string;
    path: string;
    description?: string;
  }>;
};

export function absoluteUrl(path = "/") {
  return new URL(path, siteUrl).toString();
}

export function createPageMetadata({
  title,
  description = defaultDescription,
  path,
  locale,
  alternates,
  type = "website",
  images,
}: PageMetadataInput): Metadata {
  return {
    title,
    description,
    alternates: {
      canonical: path,
      languages: alternates,
    },
    openGraph: {
      type,
      title,
      description,
      url: absoluteUrl(path),
      siteName,
      locale: locale ? openGraphLocale(locale) : undefined,
      alternateLocale: locale ? openGraphAlternateLocales(locale) : undefined,
      images,
    },
    twitter: {
      card: images && images.length > 0 ? "summary_large_image" : "summary",
      title,
      description,
      images,
    },
    other: locale
      ? {
          "content-language": htmlLang[locale],
        }
      : undefined,
  };
}

export function jsonLdScriptProps(data: unknown) {
  return {
    type: "application/ld+json",
    dangerouslySetInnerHTML: {
      __html: JSON.stringify(data).replace(/</g, "\\u003c"),
    },
  };
}

export function websiteJsonLd(locale: Locale) {
  return {
    "@context": "https://schema.org",
    "@type": "WebSite",
    "@id": `${siteUrl}/#website`,
    name: siteName,
    url: siteUrl,
    inLanguage: htmlLang[locale],
    description: defaultDescription,
    publisher: organizationJsonLd(),
  };
}

export function softwareApplicationJsonLd(locale: Locale) {
  return {
    "@context": "https://schema.org",
    "@type": "SoftwareApplication",
    "@id": `${siteUrl}/#software`,
    name: siteName,
    applicationCategory: "DeveloperApplication",
    operatingSystem: "macOS, Linux",
    url: siteUrl,
    inLanguage: htmlLang[locale],
    description: defaultDescription,
    offers: {
      "@type": "Offer",
      price: "0",
      priceCurrency: "USD",
    },
  };
}

export function articleJsonLd({
  title,
  description = defaultDescription,
  path,
  locale,
  datePublished,
  dateModified,
  author = "One CLI Team",
  tags = [],
  section,
}: ArticleJsonLdInput) {
  const url = absoluteUrl(path);

  return {
    "@context": "https://schema.org",
    "@type": "TechArticle",
    headline: title,
    description,
    url,
    mainEntityOfPage: url,
    inLanguage: htmlLang[locale],
    datePublished,
    dateModified: dateModified ?? datePublished,
    author: {
      "@type": "Organization",
      name: author,
    },
    publisher: organizationJsonLd(),
    keywords: tags,
    articleSection: section,
  };
}

export function blogPostingJsonLd(input: ArticleJsonLdInput) {
  return {
    ...articleJsonLd(input),
    "@type": "BlogPosting",
  };
}

export function breadcrumbJsonLd(items: Array<{ name: string; path: string }>) {
  return {
    "@context": "https://schema.org",
    "@type": "BreadcrumbList",
    itemListElement: items.map((item, index) => ({
      "@type": "ListItem",
      position: index + 1,
      name: item.name,
      item: absoluteUrl(item.path),
    })),
  };
}

export function itemListJsonLd({ name, description, items }: ItemListInput) {
  return {
    "@context": "https://schema.org",
    "@type": "ItemList",
    name,
    description,
    itemListElement: items.map((item, index) => ({
      "@type": "ListItem",
      position: index + 1,
      url: absoluteUrl(item.path),
      name: item.name,
      description: item.description,
    })),
  };
}

function organizationJsonLd() {
  return {
    "@type": "Organization",
    name: "1CLI Team",
    url: siteUrl,
    logo: absoluteUrl("/brand/icon.svg"),
  };
}

function openGraphLocale(locale: Locale) {
  return locale === "zh" ? "zh_CN" : "en_US";
}

function openGraphAlternateLocales(locale: Locale) {
  return locale === "zh" ? ["en_US"] : ["zh_CN"];
}
