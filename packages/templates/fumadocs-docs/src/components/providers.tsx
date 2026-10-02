"use client";
import type { ReactNode } from "react";
import { RootProvider } from "fumadocs-ui/provider/next";
import { Toaster } from "@/components/ui/toast";
import { StaticSearchDialog } from "@/components/search-dialog";
import { translations, type Locale } from "@/lib/i18n";

export function Providers({ locale, children }: { locale: Locale; children: ReactNode }) {
	return (
		<RootProvider
			search={{ SearchDialog: StaticSearchDialog }}
			i18n={{
				locale,
				defaultLanguage: "en",
				hideLocale: "never",
				locales: [
					{ locale: "en", name: "English" },
					{ locale: "zh", name: "中文" },
				],
				translations: translations[locale],
			}}
		>
			{children}
			<Toaster />
		</RootProvider>
	);
}
