"use client";

import {
  SidebarContent,
  SidebarDrawerContent,
  SidebarDrawerOverlay,
} from "fumadocs-ui/components/sidebar/base";
import type { ReactNode } from "react";

export function DocsSidebarShell({ children }: { children: ReactNode }) {
  return (
    <>
      <SidebarContent>
        {({ ref }) => (
          <aside
            ref={ref}
            id="nd-sidebar"
            className="one-docs-sidebar-shell sticky top-(--fd-docs-row-1) z-20 h-[calc(var(--fd-docs-height)-var(--fd-docs-row-1))] w-(--fd-sidebar-width) justify-self-end border-e [grid-area:sidebar] max-md:hidden"
          >
            {children}
          </aside>
        )}
      </SidebarContent>
      <SidebarDrawerOverlay className="fixed inset-0 z-40 bg-black/20 backdrop-blur-xs" />
      <SidebarDrawerContent className="one-docs-sidebar-mobile fixed inset-y-0 end-0 z-50 w-[85%] max-w-[380px] overflow-y-auto border-s shadow-lg [--fd-sidebar-width:100%]">
        {children}
      </SidebarDrawerContent>
    </>
  );
}
