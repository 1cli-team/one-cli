import { fileURLToPath } from "node:url";
import { createMDX } from "fumadocs-mdx/next";

const withMDX = createMDX();
const workspaceRoot = fileURLToPath(new URL("../..", import.meta.url));

/** @type {import('next').NextConfig} */
const config = {
  // Keep build artifacts under apps/docs/dist while letting Vercel run the
  // managed Next.js runtime, including the Image Optimization API.
  distDir: "dist",
  outputFileTracingRoot: workspaceRoot,
  // 让生成的 URL 与 Starlight 的 trailing-slash 行为一致。
  trailingSlash: true,
  reactStrictMode: true,
};

export default withMDX(config);
