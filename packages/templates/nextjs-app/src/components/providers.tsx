"use client";

import type { PropsWithChildren } from "react";
import { SWRConfig } from "swr";
import { ThemeProvider } from "@/components/theme-provider";
import { Toaster } from "@/components/ui/toast";
import { UIProvider } from "@/stores/ui";
import { fetcher } from "@/lib/http";

export function Providers({ children }: PropsWithChildren) {
	return (
		<UIProvider>
			<ThemeProvider attribute="class" defaultTheme="system" enableSystem disableTransitionOnChange>
				<SWRConfig value={{ fetcher }}>
					<Toaster>{children}</Toaster>
				</SWRConfig>
			</ThemeProvider>
		</UIProvider>
	);
}
