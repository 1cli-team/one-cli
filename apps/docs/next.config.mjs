import { createMDX } from "fumadocs-mdx/next";

const withMDX = createMDX();

/** @type {import('next').NextConfig} */
const config = {
  // SSG: 全静态导出，交给 Vercel 作为静态 Next.js 文档站托管。
  output: "export",
  // Vercel 的 Output Directory 指向 apps/docs/dist。
  distDir: "dist",
  // SSG 模式下 next/image 默认依赖 server，关掉优化用原图。
  images: { unoptimized: true },
  // 让生成的 URL 与 Starlight 的 trailing-slash 行为一致。
  trailingSlash: true,
  reactStrictMode: true,
};

export default withMDX(config);
