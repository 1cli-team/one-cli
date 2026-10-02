import type { Metadata } from "next";
import type { ReactNode } from "react";
import { Providers } from "@/components/providers";
import "./globals.css";

export const metadata: Metadata = {
	title: "One CLI · Next.js",
	description: "Next.js application starter / Next.js 应用模板",
};
export default function RootLayout({ children }: { children: ReactNode }) {
	return (
		<html lang="en" suppressHydrationWarning>
			<body className="antialiased">
				<Providers>{children}</Providers>
			</body>
		</html>
	);
}
