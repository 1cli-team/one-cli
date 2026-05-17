import type { LocalizedText } from "@/data/examples";

export type TemplateKind = "frontend" | "backend" | "library";

export type DeployOption = {
  code: string;
  id: string;
  name: LocalizedText;
};

export type TemplateMeta = {
  id: string;
  code: string;
  kind: TemplateKind;
  presetKind: "f" | "b" | "l";
  title: LocalizedText;
  tagline: LocalizedText;
  toolchain: "node" | "go";
  defaultName: string;
  tags: string[];
  /** Empty array = template forbids a deploy code (e.g. expo / electron / library). */
  deployOptions: DeployOption[];
  /** Code of the default deploy option, or null if the template has no deploy. */
  defaultDeployCode: string | null;
};

const DEPLOY_LIBRARY: Record<string, LocalizedText> = {
  kustomize: { zh: "Kustomize (k8s)", en: "Kustomize (k8s)" },
  vercel: { zh: "Vercel", en: "Vercel" },
  cloudflare: { zh: "Cloudflare Pages", en: "Cloudflare Pages" },
  "aws-s3": { zh: "AWS S3 静态托管", en: "AWS S3 static" },
  "aliyun-oss": { zh: "阿里云 OSS", en: "Aliyun OSS" },
  "tencent-cos": { zh: "腾讯云 COS", en: "Tencent COS" },
  r2: { zh: "Cloudflare R2", en: "Cloudflare R2" },
  minio: { zh: "MinIO", en: "MinIO" },
  rustfs: { zh: "RustFS", en: "RustFS" },
  edgeone: { zh: "腾讯云 EdgeOne", en: "Tencent EdgeOne" },
};

const DEPLOY_CODE: Record<string, string> = {
  kustomize: "k",
  vercel: "v",
  cloudflare: "c",
  "aws-s3": "s",
  "aliyun-oss": "a",
  "tencent-cos": "t",
  r2: "2",
  minio: "m",
  rustfs: "r",
  edgeone: "e",
};

function deploy(ids: string[]): DeployOption[] {
  return ids.map((id) => ({
    id,
    code: DEPLOY_CODE[id] ?? "",
    name: DEPLOY_LIBRARY[id] ?? { zh: id, en: id },
  }));
}

export const templates: TemplateMeta[] = [
  {
    id: "nestjs-api",
    code: "ne",
    kind: "backend",
    presetKind: "b",
    title: { zh: "NestJS API", en: "NestJS API" },
    tagline: {
      zh: "NestJS + TypeScript，适合业务后台",
      en: "NestJS + TypeScript, ideal for business APIs",
    },
    toolchain: "node",
    defaultName: "api",
    tags: ["api", "nestjs", "typescript"],
    deployOptions: deploy(["kustomize"]),
    defaultDeployCode: "k",
  },
  {
    id: "go-api",
    code: "go",
    kind: "backend",
    presetKind: "b",
    title: { zh: "Go API", en: "Go API" },
    tagline: {
      zh: "Gin + Gorm + Viper + Zap，适合高并发服务",
      en: "Gin + Gorm + Viper + Zap, ready for high-traffic services",
    },
    toolchain: "go",
    defaultName: "api",
    tags: ["api", "go", "kustomize"],
    deployOptions: deploy(["kustomize"]),
    defaultDeployCode: "k",
  },
  {
    id: "nextjs-app",
    code: "na",
    kind: "frontend",
    presetKind: "f",
    title: { zh: "Next.js 应用", en: "Next.js App" },
    tagline: {
      zh: "App Router + SSR，SEO 友好，可部署到 Vercel / Cloudflare / k8s",
      en: "App Router + SSR, SEO-friendly, deployable to Vercel / Cloudflare / k8s",
    },
    toolchain: "node",
    defaultName: "web",
    tags: ["web", "nextjs", "react"],
    deployOptions: deploy(["kustomize", "vercel", "cloudflare"]),
    defaultDeployCode: "k",
  },
  {
    id: "react-spa",
    code: "rs",
    kind: "frontend",
    presetKind: "f",
    title: { zh: "React SPA", en: "React SPA" },
    tagline: {
      zh: "React + Vite + React Router，适合后台与交互应用",
      en: "React + Vite + Router, great for admin and interactive apps",
    },
    toolchain: "node",
    defaultName: "web",
    tags: ["web", "vite", "react"],
    deployOptions: deploy([
      "aws-s3",
      "cloudflare",
      "vercel",
      "r2",
      "aliyun-oss",
      "tencent-cos",
      "minio",
      "rustfs",
      "edgeone",
    ]),
    defaultDeployCode: "s",
  },
  {
    id: "astro-site",
    code: "as",
    kind: "frontend",
    presetKind: "f",
    title: { zh: "Astro 站点", en: "Astro Site" },
    tagline: {
      zh: "MDX 内容集合，适合官网、内容站",
      en: "MDX content collections, great for marketing and content sites",
    },
    toolchain: "node",
    defaultName: "site",
    tags: ["web", "astro", "content"],
    deployOptions: deploy([
      "aws-s3",
      "cloudflare",
      "vercel",
      "r2",
      "aliyun-oss",
      "tencent-cos",
      "minio",
      "rustfs",
      "edgeone",
    ]),
    defaultDeployCode: "s",
  },
  {
    id: "starlight-docs",
    code: "sl",
    kind: "frontend",
    presetKind: "f",
    title: { zh: "Starlight 文档", en: "Starlight Docs" },
    tagline: {
      zh: "Astro Starlight 文档站，自带搜索",
      en: "Astro Starlight docs with built-in search",
    },
    toolchain: "node",
    defaultName: "docs",
    tags: ["docs", "starlight", "astro"],
    deployOptions: deploy([
      "aws-s3",
      "cloudflare",
      "vercel",
      "r2",
      "aliyun-oss",
      "tencent-cos",
      "minio",
      "rustfs",
      "edgeone",
    ]),
    defaultDeployCode: "s",
  },
  {
    id: "electron-app",
    code: "ea",
    kind: "frontend",
    presetKind: "f",
    title: { zh: "Electron 桌面", en: "Electron Desktop" },
    tagline: {
      zh: "Electron + React + Vite + inversify",
      en: "Electron + React + Vite + inversify",
    },
    toolchain: "node",
    defaultName: "desktop",
    tags: ["desktop", "electron", "react"],
    deployOptions: [],
    defaultDeployCode: null,
  },
  {
    id: "expo-mobile",
    code: "em",
    kind: "frontend",
    presetKind: "f",
    title: { zh: "Expo 移动 App", en: "Expo Mobile" },
    tagline: {
      zh: "Expo + React Native，双端 App",
      en: "Expo + React Native, iOS / Android",
    },
    toolchain: "node",
    defaultName: "mobile",
    tags: ["mobile", "expo", "native"],
    deployOptions: [],
    defaultDeployCode: null,
  },
  {
    id: "ts-library",
    code: "tl",
    kind: "library",
    presetKind: "l",
    title: { zh: "TypeScript 库", en: "TS Library" },
    tagline: {
      zh: "tsdown + vitest + 严格公开 API",
      en: "tsdown + vitest with a strict public API",
    },
    toolchain: "node",
    defaultName: "shared",
    tags: ["library", "typescript", "package"],
    deployOptions: [],
    defaultDeployCode: null,
  },
  {
    id: "go-lib",
    code: "gl",
    kind: "library",
    presetKind: "l",
    title: { zh: "Go 库", en: "Go Library" },
    tagline: {
      zh: "遵循 golang-standards/project-layout，发包与 import 兼用",
      en: "golang-standards/project-layout, ready to publish and be imported",
    },
    toolchain: "go",
    defaultName: "lib",
    tags: ["library", "go", "golang", "module"],
    deployOptions: [],
    defaultDeployCode: null,
  },
];

export function getTemplateById(id: string): TemplateMeta | null {
  return templates.find((t) => t.id === id) ?? null;
}
