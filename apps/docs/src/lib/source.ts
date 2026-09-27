import { docs, tutorials } from "../../.source/server";
import { loader } from "fumadocs-core/source";
import { icons } from "lucide-react";
import { createElement } from "react";
import { i18n } from "@/i18n";

function makeIconResolver() {
  return (icon?: string) => {
    if (!icon) return;
    if (icon in icons) {
      return createElement(icons[icon as keyof typeof icons]);
    }
  };
}

export const source = loader({
  baseUrl: "/docs",
  i18n,
  source: docs.toFumadocsSource(),
  icon: makeIconResolver(),
});

// 教程是独立 fumadocs source：URL `/tutorials/*`，内容根目录 `content/tutorials/`。
// 和 docs source 完全解耦，sidebar / breadcrumb / generateStaticParams 都各走各的。
export const tutorialsSource = loader({
  baseUrl: "/tutorials",
  i18n,
  source: tutorials.toFumadocsSource(),
  icon: makeIconResolver(),
});
