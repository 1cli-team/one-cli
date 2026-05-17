import { defineConfig, defineDocs } from "fumadocs-mdx/config";

// content/docs/ 下的所有 .md / .mdx 都被 docs collection 收纳。
// 默认会同时找 meta.json (sidebar 排序) 和 frontmatter (title / description)。
export const docs = defineDocs({
  dir: "content/docs",
});

// content/tutorials/ 是教程独立树，URL 挂在 /tutorials/* 而不是 /docs/*。
export const tutorials = defineDocs({
  dir: "content/tutorials",
});

export default defineConfig({
  mdxOptions: {
    // Shiki theme: github-dark 与 DESIGN.md 的 dark navy code-window-card 对齐
    rehypeCodeOptions: {
      themes: {
        light: "github-dark",
        dark: "github-dark",
      },
    },
  },
});
