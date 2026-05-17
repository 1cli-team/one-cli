import "./global.css";
import { RootProvider } from "fumadocs-ui/provider";
import { ViewTransitions } from "next-view-transitions";
import Script from "next/script";
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
      <html lang="en" suppressHydrationWarning className="font-sans">
        <head>
          <Script id="html-lang-by-locale" strategy="beforeInteractive">
            {`(() => {
  const path = window.location.pathname;
  const lang = path === "/zh" || path.startsWith("/zh/") ? "zh-Hans" : "en";
  document.documentElement.lang = lang;
})();`}
          </Script>
        </head>
        <body className="flex flex-col min-h-screen">
          <RootProvider theme={{ enabled: false }}>{children}</RootProvider>
        </body>
      </html>
    </ViewTransitions>
  );
}
