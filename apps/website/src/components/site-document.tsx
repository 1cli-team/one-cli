import type { Metadata } from "next";
import type { ReactNode } from "react";
import { htmlLang, type Locale } from "@/i18n";
import { defaultDescription, siteName, siteUrl } from "@/lib/seo";
import "@/app/globals.css";

export const siteMetadata: Metadata = {
	metadataBase: new URL(siteUrl),
	title: siteName,
	description: defaultDescription,
	icons: {
		apple: "/brand/icon.svg",
		icon: "/brand/favicon.svg",
		shortcut: "/brand/favicon.svg",
	},
};

export function SiteDocument({ locale, children }: { locale: Locale; children: ReactNode }) {
	return (
		<html lang={htmlLang[locale]} className="font-sans">
			<body className="flex flex-col min-h-screen">{children}</body>
		</html>
	);
}
