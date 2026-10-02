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
  // 使用目录式 URL，便于静态托管与直接访问文档链接。
  trailingSlash: true,
  reactStrictMode: true,
  async redirects() {
    const movedPages = [
      ["docs/configure", "docs/login"],
      ["tutorials/configure-profiles", "tutorials/infisical-login"],
      ["tutorials/run-passthrough", "docs/exec"],
    ];

    return [
      {
        source: "/:lang(zh|en)/docs/",
        destination: "/:lang/docs/quick-start/",
        permanent: true,
      },
      {
        source: "/:lang(zh|en)/tutorials/",
        destination: "/:lang/tutorials/templates/",
        permanent: true,
      },
      ...movedPages.flatMap(([from, to]) => [
        {
          source: `/:lang(zh|en)/${from}/`,
          destination: `/:lang/${to}/`,
          permanent: true,
        },
        {
          source: `/${from}/`,
          destination: `/zh/${to}/`,
          permanent: true,
        },
      ]),
    ];
  },
};

export default withMDX(config);
