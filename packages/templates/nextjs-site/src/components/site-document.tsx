import type { ReactNode } from "react";
import { ThemeProvider } from "@/components/theme-provider";
import { Toaster } from "@/components/ui/toast";
import { htmlLang, type Locale } from "@/lib/i18n";
import "@/app/globals.css";

export function SiteDocument({ locale, children }: { locale: Locale; children: ReactNode }) {
	return (
		<html lang={htmlLang[locale]} suppressHydrationWarning>
			<body className="min-h-screen font-sans antialiased">
				<ThemeProvider
					attribute="class"
					defaultTheme="system"
					enableSystem
					disableTransitionOnChange
				>
					{children}
					<Toaster />
				</ThemeProvider>
			</body>
		</html>
	);
}
