import "./global.css";
import { RootProvider } from "fumadocs-ui/provider";
import { ViewTransitions } from "next-view-transitions";
import type { ReactNode } from "react";
import { defaultDescription, siteName, siteUrl } from "@/lib/seo";

export const metadata = {
  metadataBase: new URL(siteUrl),
  title: siteName,
  description: defaultDescription,
  icons: {
    apple: "/brand/icon.svg",
    icon: "/brand/favicon.svg",
    shortcut: "/brand/favicon.svg",
  },
  openGraph: {
    siteName,
    type: "website",
    url: siteUrl,
    title: siteName,
    description: defaultDescription,
  },
  twitter: {
    card: "summary",
    title: siteName,
    description: defaultDescription,
  },
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <ViewTransitions>
      <html lang="zh-Hans" suppressHydrationWarning className="font-sans">
        <body className="flex flex-col min-h-screen">
          <RootProvider theme={{ enabled: false }}>{children}</RootProvider>
        </body>
      </html>
    </ViewTransitions>
  );
}
