import type { ReactNode } from "react";
import { Providers } from "@/components/providers";
import { htmlLang, type Locale } from "@/lib/i18n";
import "@/app/globals.css";
export function SiteDocument({ locale, children }: { locale: Locale; children: ReactNode }) {
	return (
		<html lang={htmlLang[locale]} suppressHydrationWarning>
			<body className="flex min-h-screen flex-col font-sans antialiased">
				<Providers locale={locale}>{children}</Providers>
			</body>
		</html>
	);
}
