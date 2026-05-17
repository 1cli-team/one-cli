import "./global.css";
import { RootProvider } from "fumadocs-ui/provider";
import { ViewTransitions } from "next-view-transitions";
import type { ReactNode } from "react";
import { Geist } from "next/font/google";
import { cn } from "@/lib/utils";

const geist = Geist({subsets:['latin'],variable:'--font-sans'});

export const metadata = {
  metadataBase: new URL("https://one.torchstellar.com"),
  title: "One CLI",
  description: "面向团队工作区的脚手架与治理工具",
  icons: {
    apple: "/brand/icon.svg",
    icon: "/brand/favicon.svg",
    shortcut: "/brand/favicon.svg",
  },
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <ViewTransitions>
      <html lang="zh-Hans" suppressHydrationWarning className={cn("font-sans", geist.variable)}>
        <head>
          <link rel="preconnect" href="https://fonts.googleapis.com" />
          <link
            rel="preconnect"
            href="https://fonts.gstatic.com"
            crossOrigin=""
          />
          <link
            rel="stylesheet"
            href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&family=Geist:wght@400;500;600&family=Funnel+Sans:wght@500;600;700&family=JetBrains+Mono:wght@400;500;600&display=swap"
          />
        </head>
        <body className="flex flex-col min-h-screen">
          <RootProvider theme={{ enabled: false }}>{children}</RootProvider>
        </body>
      </html>
    </ViewTransitions>
  );
}
