"use client";

import { DocsSidebarShell } from "./docs-sidebar-shell";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { localizedTutorialsPath, type Locale } from "@/i18n";

type SidebarItem = {
  label: string;
  href: string;
};

type SidebarSection = {
  label: string;
  items: SidebarItem[];
};

const sectionsByLocale: Record<Locale, SidebarSection[]> = {
  zh: [
    {
      label: "基础教程",
      items: [
        { label: "浏览项目模板", href: "/tutorials/templates/" },
        { label: "手动创建工作区", href: "/tutorials/first-workspace/" },
      ],
    },
    {
      label: "进阶教程",
      items: [
        { label: "登录与共享凭据", href: "/tutorials/infisical-login/" },
        { label: "配置环境变量", href: "/tutorials/env-vars/" },
        { label: "多环境变量", href: "/tutorials/env-multi-env/" },
        { label: "本地开发编排", href: "/tutorials/dev-local/" },
        {
          label: "输出与错误码",
          href: "/tutorials/json-output-error-codes/",
        },
      ],
    },
  ],
  en: [
    {
      label: "BASICS",
      items: [
        { label: "Browse project templates", href: "/tutorials/templates/" },
        { label: "Manual workspace", href: "/tutorials/first-workspace/" },
      ],
    },
    {
      label: "ADVANCED",
      items: [
        {
          label: "Login and shared credentials",
          href: "/tutorials/infisical-login/",
        },
        { label: "Configure env vars", href: "/tutorials/env-vars/" },
        { label: "Multi-env vars", href: "/tutorials/env-multi-env/" },
        { label: "Local dev orchestration", href: "/tutorials/dev-local/" },
        {
          label: "Output & error codes",
          href: "/tutorials/json-output-error-codes/",
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
    ariaLabel: "教程侧边栏",
  },
  en: {
    ariaLabel: "Tutorials sidebar",
  },
};

export function TutorialsSidebar({ lang }: { lang: Locale }) {
  return (
    <DocsSidebarShell>
      <TutorialsSidebarInner lang={lang} />
    </DocsSidebarShell>
  );
}

function TutorialsSidebarInner({ lang }: { lang: Locale }) {
  const pathname = usePathname();
  const labels = sidebarText[lang];
  const sections = sectionsByLocale[lang];

  return (
    <div className="one-docs-sidebar-inner">
      <nav className="one-docs-side-nav" aria-label={labels.ariaLabel}>
        {sections.map((section, sectionIdx) => (
          <section
            className="one-docs-side-section"
            data-unlabeled={section.label ? undefined : true}
            key={`section-${sectionIdx}`}
          >
            {section.label ? <h2>{section.label}</h2> : null}
            <div className="one-docs-side-list">
              {section.items.map((item) => {
                const slugParts = item.href
                  .replace(/^\/tutorials\/?/, "")
                  .replace(/\/$/, "")
                  .split("/")
                  .filter(Boolean);
                const href = localizedTutorialsPath(lang, slugParts);
                const active = isActive(pathname, href);

                return (
                  <Link
                    aria-current={active ? "page" : undefined}
                    className="one-docs-side-item"
                    data-active={active}
                    href={href}
                    key={`${sectionIdx}:${item.href}:${item.label}`}
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
  // Section root (e.g. `/zh/tutorials/`) should match only on exact equality —
  // otherwise it would prefix-match every tutorial page.
  const segments = href.split("/").filter(Boolean).length;
  if (segments <= 2) return false;
  const normalizedPath = pathname.endsWith("/") ? pathname : `${pathname}/`;
  return normalizedPath.startsWith(href);
}
