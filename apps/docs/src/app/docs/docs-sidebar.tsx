"use client";

import {
  Sidebar,
  SidebarContent,
  SidebarContentMobile,
} from "fumadocs-ui/components/layout/sidebar";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { localizedDocsPath, type Locale } from "@/i18n";

type SidebarItem = {
  label: string;
  href: string;
  mono?: boolean;
};

type SidebarSection = {
  label: string;
  items: SidebarItem[];
};

const sectionsByLocale: Record<Locale, SidebarSection[]> = {
  zh: [
    {
      label: "开始使用",
      items: [
        { label: "安装", href: "/docs/installation/" },
        { label: "快速开始", href: "/docs/quick-start/" },
      ],
    },
    {
      label: "核心概念",
      items: [
        { label: "工作区清单", href: "/docs/manifest/" },
        { label: "模板", href: "/docs/templates/" },
        { label: "治理规则", href: "/docs/ai-native/" },
      ],
    },
    {
      label: "CLI 命令",
      items: [
        { label: "命令总览", href: "/docs/cli-overview/" },
        { label: "one create", href: "/docs/create/", mono: true },
        { label: "one add", href: "/docs/add/", mono: true },
        { label: "one env", href: "/docs/env-vars/", mono: true },
        {
          label: "one configure",
          href: "/docs/configure/",
          mono: true,
        },
        {
          label: "one templates",
          href: "/docs/templates-cmd/",
          mono: true,
        },
        {
          label: "one container",
          href: "/docs/container/",
          mono: true,
        },
        { label: "one dev", href: "/docs/dev/", mono: true },
        { label: "one deploy", href: "/docs/deploy/", mono: true },
        { label: "one run", href: "/docs/run/", mono: true },
        { label: "Skills", href: "/docs/skills/" },
        { label: "one serve", href: "/docs/serve/", mono: true },
        { label: "错误码", href: "/docs/error-codes/" },
      ],
    },
  ],
  en: [
    {
      label: "GETTING STARTED",
      items: [
        { label: "Installation", href: "/docs/installation/" },
        { label: "Quick start", href: "/docs/quick-start/" },
      ],
    },
    {
      label: "CORE CONCEPTS",
      items: [
        { label: "Workspace manifest", href: "/docs/manifest/" },
        { label: "Templates", href: "/docs/templates/" },
        { label: "Governance rules", href: "/docs/ai-native/" },
      ],
    },
    {
      label: "CLI COMMANDS",
      items: [
        { label: "Command overview", href: "/docs/cli-overview/" },
        { label: "one create", href: "/docs/create/", mono: true },
        { label: "one add", href: "/docs/add/", mono: true },
        { label: "one env", href: "/docs/env-vars/", mono: true },
        {
          label: "one configure",
          href: "/docs/configure/",
          mono: true,
        },
        {
          label: "one templates",
          href: "/docs/templates-cmd/",
          mono: true,
        },
        {
          label: "one container",
          href: "/docs/container/",
          mono: true,
        },
        { label: "one dev", href: "/docs/dev/", mono: true },
        { label: "one deploy", href: "/docs/deploy/", mono: true },
        { label: "one run", href: "/docs/run/", mono: true },
        { label: "Skills", href: "/docs/skills/" },
        { label: "one serve", href: "/docs/serve/", mono: true },
        {
          label: "error codes",
          href: "/docs/error-codes/",
          mono: true,
        },
      ],
    },
  ],
};

const sidebarText: Record<
  Locale,
  {
    ariaLabel: string;
  }
> = {
  zh: {
    ariaLabel: "文档侧边栏",
  },
  en: {
    ariaLabel: "Docs sidebar",
  },
};

export function DocsSidebar({ lang }: { lang: Locale }) {
  const sidebar = <DocsSidebarInner lang={lang} />;

  return (
    <Sidebar
      Content={
        <SidebarContent className="one-docs-sidebar-shell">
          {sidebar}
        </SidebarContent>
      }
      Mobile={
        <SidebarContentMobile className="one-docs-sidebar-mobile">
          {sidebar}
        </SidebarContentMobile>
      }
    />
  );
}

function DocsSidebarInner({ lang }: { lang: Locale }) {
  const pathname = usePathname();
  const labels = sidebarText[lang];
  const sections = sectionsByLocale[lang];

  return (
    <div className="one-docs-sidebar-inner">
      <nav className="one-docs-side-nav" aria-label={labels.ariaLabel}>
        {sections.map((section) => (
          <section className="one-docs-side-section" key={section.label}>
            <h2>{section.label}</h2>
            <div className="one-docs-side-list">
              {section.items.map((item) => {
                const href = localizedDocsPath(
                  lang,
                  item.href
                    .replace(/^\/docs\/?/, "")
                    .replace(/\/$/, "")
                    .split("/")
                    .filter(Boolean),
                );
                const active = isActive(pathname, href);

                return (
                  <Link
                    aria-current={active ? "page" : undefined}
                    className="one-docs-side-item"
                    data-active={active}
                    data-mono={item.mono ? "true" : undefined}
                    href={href}
                    key={`${section.label}:${item.href}:${item.label}`}
                  >
                    {item.label}
                  </Link>
                );
              })}
            </div>
          </section>
        ))}
      </nav>
    </div>
  );
}

function isActive(pathname: string, href: string) {
  if (pathname === href) return true;
  const normalizedPath = pathname.endsWith("/") ? pathname : `${pathname}/`;
  return href !== "/docs/" && normalizedPath.startsWith(href);
}
