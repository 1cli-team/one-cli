"use client";

import { PanelLeft } from "lucide-react";
import { SiteTopNav, type SiteTopNavActive } from "@/components/site-top-nav";
import type { Locale } from "@/i18n";
import {
  DocsLayout as FumadocsLayout,
  type DocsLayoutProps,
  type DocsSlots,
} from "fumadocs-ui/layouts/docs";
import {
  SidebarProvider,
  SidebarTrigger,
  useSidebar,
  type SidebarProps,
} from "fumadocs-ui/layouts/docs/slots/sidebar";

function CustomSidebar({ children }: SidebarProps) {
  return children;
}

const sidebarSlot: DocsSlots["sidebar"] = {
  provider: SidebarProvider,
  root: CustomSidebar,
  trigger: SidebarTrigger,
  useSidebar,
};

export function DocsLayout({
  lang,
  active,
  ...props
}: DocsLayoutProps & { lang: Locale; active: SiteTopNavActive }) {
  const menuLabel = lang === "zh" ? "目录" : "Contents";

  return (
    <FumadocsLayout
      {...props}
      nav={{
        ...props.nav,
        component: (
          <SiteTopNav
            lang={lang}
            active={active}
            sidebarTrigger={
              <SidebarTrigger
                className="one-docs-menu-trigger"
                aria-label={lang === "zh" ? "切换目录" : "Toggle contents"}
              >
                <PanelLeft aria-hidden="true" className="size-4" />
                <span>{menuLabel}</span>
              </SidebarTrigger>
            }
          />
        ),
      }}
      slots={{ sidebar: sidebarSlot }}
    />
  );
}
