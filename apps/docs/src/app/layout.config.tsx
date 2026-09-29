import type { BaseLayoutProps } from "fumadocs-ui/layouts/shared";
import { BrandMark } from "@/components/brand-mark";

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
  ],
};
