/** @type {import("next").NextConfig} */
const config = {
  output: "export",
  experimental: { globalNotFound: true },
  trailingSlash: true,
  images: { unoptimized: true },
};

export default config;
