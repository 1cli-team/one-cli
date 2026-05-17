import type { BaseLayoutProps } from "fumadocs-ui/layouts/shared";
import { BrandMark } from "@/components/brand-mark";

// 顶部 nav 共享配置：左边是 brand mark；右边的
// 链接接 Docs / GitHub。"Get started" 这个 pill CTA 通过 layout 自带
// 的 actions slot 由 home/docs 各自的 layout 注入，避免在 docs 内
// 看到与文档无关的营销 CTA。
export const baseOptions: BaseLayoutProps = {
  nav: {
    title: <BrandMark variant="light" />,
  },
  links: [
    {
      text: "Tutorials",
      url: "/zh/tutorials/templates/",
      active: "nested-url",
    },
    {
      text: "Docs",
      url: "/zh/docs/quick-start/",
      active: "nested-url",
    },
    { text: "Templates", url: "/zh/templates/", active: "nested-url" },
    { text: "Blog", url: "/zh/blog/", active: "nested-url" },
    {
      text: "Changelog",
      url: "https://github.com/1cli-team/one-cli/blob/master/CHANGELOG.md",
      external: true,
    },
  ],
};
