import { createMDX } from "fumadocs-mdx/next";
const withMDX = createMDX();

/** @type {import("next").NextConfig} */
const config = {
  output: "export",
  experimental: { globalNotFound: true },
  trailingSlash: true,
  images: { unoptimized: true },
};

export default withMDX(config);
