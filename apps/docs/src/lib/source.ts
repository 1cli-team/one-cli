import { docs, tutorials } from "../../.source";
import { loader } from "fumadocs-core/source";
import { icons } from "lucide-react";
import { createElement } from "react";
import { i18n } from "@/i18n";

// fumadocs-mdx@11.10.1 .toFumadocsSource() 返回的是 `{ files: () => [...] }`
// (function)，但 fumadocs-core@15.8.5 的 loader 把 `source.files` 当数组用。
// TypeScript 用 fumadocs-core 的类型推断 (files: VirtualFile[])，所以这里强转
// 一下 unknown 再判断 runtime 形状。
function resolveFiles(input: {
  toFumadocsSource: () => unknown;
}): ReadonlyArray<unknown> {
  const fumadocsSource = input.toFumadocsSource() as unknown as {
    files: ReadonlyArray<unknown> | (() => ReadonlyArray<unknown>);
  };
  return typeof fumadocsSource.files === "function"
    ? fumadocsSource.files()
    : fumadocsSource.files;
}

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
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  source: { files: resolveFiles(docs) as any },
  icon: makeIconResolver(),
});

// 教程是独立 fumadocs source：URL `/tutorials/*`，内容根目录 `content/tutorials/`。
// 和 docs source 完全解耦，sidebar / breadcrumb / generateStaticParams 都各走各的。
export const tutorialsSource = loader({
  baseUrl: "/tutorials",
  i18n,
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  source: { files: resolveFiles(tutorials) as any },
  icon: makeIconResolver(),
});
