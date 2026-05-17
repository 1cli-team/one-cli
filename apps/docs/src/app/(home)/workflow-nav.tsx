"use client";

import { useEffect, useState } from "react";
import {
  Code2,
  FileJson2,
  Layers3,
  Route,
  Sparkles,
  Wrench,
} from "lucide-react";

export type WorkflowNavIcon =
  | "code"
  | "layers"
  | "wrench"
  | "file-json"
  | "route"
  | "sparkles";

type WorkflowNavItem = {
  command: string;
  navBody: string;
  icon: WorkflowNavIcon;
  slug: string;
};

const workflowIcons = {
  code: Code2,
  layers: Layers3,
  wrench: Wrench,
  "file-json": FileJson2,
  route: Route,
  sparkles: Sparkles,
} as const;

export function WorkflowSidebarNav({ items }: { items: WorkflowNavItem[] }) {
  const [activeSlug, setActiveSlug] = useState(items[0]?.slug ?? "");

  useEffect(() => {
    const updateActiveSlug = () => {
      let nextSlug = items[0]?.slug ?? "";

      for (const item of items) {
        const section = document.getElementById(item.slug);
        if (!section) continue;

        if (section.getBoundingClientRect().top <= 220) {
          nextSlug = item.slug;
        }
      }

      setActiveSlug(nextSlug);
    };

    updateActiveSlug();
    window.addEventListener("scroll", updateActiveSlug, { passive: true });
    window.addEventListener("hashchange", updateActiveSlug);

    return () => {
      window.removeEventListener("scroll", updateActiveSlug);
      window.removeEventListener("hashchange", updateActiveSlug);
    };
  }, [items]);

  return (
    <nav className="flex flex-col gap-2 px-8 py-8">
      {items.map(({ command, navBody, icon, slug }) => {
        const Icon = workflowIcons[icon];
        const active = slug === activeSlug;

        return (
          <a
            aria-current={active ? "true" : undefined}
            className={[
              "group flex min-h-[68px] items-center gap-4 rounded-md px-3 py-3 transition",
              active
                ? "bg-white/[0.04] text-white"
                : "text-stone-400 hover:bg-white/[0.03] hover:text-white",
            ].join(" ")}
            href={`#${slug}`}
            key={command}
          >
            <span
              className={[
                "flex size-8 shrink-0 items-center justify-center rounded-md transition",
                active
                  ? "bg-[#1c1917] text-[#ea580c]"
                  : "bg-[#1c1917]/70 text-stone-400 group-hover:text-stone-200",
              ].join(" ")}
            >
              <Icon className="size-4" />
            </span>
            <span className="min-w-0">
              <span className="block truncate font-mono text-sm font-semibold">
                {command}
              </span>
              <span className="mt-0.5 block text-xs leading-5 text-stone-400/80">
                {navBody}
              </span>
            </span>
          </a>
        );
      })}
    </nav>
  );
}
