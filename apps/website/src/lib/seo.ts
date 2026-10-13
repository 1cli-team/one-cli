import type { Metadata } from "next";
import { htmlLang, type Locale } from "@/i18n";

export const siteUrl = "https://www.1cli.dev";
export const siteName = "One CLI";
export const defaultDescription =
	"One CLI helps people and AI create workspaces, run project tasks, and manage variables and development services.";

type PageMetadataInput = {
	title: string;
	description?: string;
	path: string;
	locale?: Locale;
	alternates?: Record<string, string>;
	type?: "website" | "article";
	images?: string[];
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
		operatingSystem: "macOS, Linux, Windows",
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
