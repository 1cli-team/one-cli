"use client";

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

export function DocsLayout(props: DocsLayoutProps) {
  return <FumadocsLayout {...props} slots={{ sidebar: sidebarSlot }} />;
}
