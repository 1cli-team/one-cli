import { createMDX } from "fumadocs-mdx/next";

const withMDX = createMDX();

/** @type {import('next').NextConfig} */
const config = {
  // SSG: 全静态导出，对接 website-deploy.yml 现有 OSS 上传逻辑。
  output: "export",
  // CI 上传 docs/dist/ 到 OSS（见 .github/workflows/website-deploy.yml）。
  distDir: "dist",
  // SSG 模式下 next/image 默认依赖 server，关掉优化用原图。
  images: { unoptimized: true },
  // 让生成的 URL 与 Starlight 的 trailing-slash 行为一致。
  trailingSlash: true,
  reactStrictMode: true,
};

export default withMDX(config);
