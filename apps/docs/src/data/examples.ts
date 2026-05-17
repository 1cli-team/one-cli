export type ExampleCategory =
  | "mobile"
  | "desktop"
  | "web"
  | "docs"
  | "consumer"
  | "admin";

export type LocalizedText = { zh: string; en: string };

export type Example = {
  id: string;
  category: ExampleCategory;
  title: LocalizedText;
  /** 合并后的"技术栈 + 适用场景"一句话，用在卡片副标题与详情页 hero。 */
  tagline: LocalizedText;
  baseTemplates: string[];
  presetId: string;
  workspaceName: string;
  cover: string;
  prompt: LocalizedText;
  tags: string[];
};

const PLACEHOLDER = "/examples/_placeholder.svg";

function makePrompt(input: {
  titleZh: string;
  titleEn: string;
  workspaceName: string;
  presetId: string;
  stackZh: string;
  stackEn: string;
}): LocalizedText {
  const zh = [
    `请使用 one-cli skill 帮我搭建${input.titleZh}。`,
    "",
    "【一键脚手架】",
    `  one create ${input.workspaceName} --preset ${input.presetId} --yes -o json`,
    `这会按 preset 一次性创建 workspace（${input.stackZh}）。`,
    "请按照 SKILL.md 的标准流程执行，并校验 one.manifest.json。",
    "",
    "脚手架就绪后告诉我，我会根据需要补充业务逻辑。",
  ].join("\n");

  const en = [
    `Use the one-cli skill to scaffold ${input.titleEn}.`,
    "",
    "[Scaffold in one shot]",
    `  one create ${input.workspaceName} --preset ${input.presetId} --yes -o json`,
    `This applies the preset (${input.stackEn}).`,
    "Follow the SKILL.md workflow and verify one.manifest.json.",
    "",
    "Let me know when the scaffold is ready and I'll layer business logic on top.",
  ].join("\n");

  return { zh, en };
}

export const examples: Example[] = [
  {
    id: "mobile-starter",
    category: "mobile",
    title: { zh: "移动端起步套件", en: "Mobile Starter" },
    tagline: {
      zh: "Expo + React Native + NestJS API，适合需要服务端配套的双端 App",
      en: "Expo + React Native + NestJS API, for mobile apps that need a backend",
    },
    baseTemplates: ["expo-mobile", "nestjs-api"],
    presetId: "1.bnekh.fem.ed",
    workspaceName: "mobile-app",
    cover: "/examples/mobile-starter/cover.png",
    prompt: makePrompt({
      titleZh: "移动端起步套件",
      titleEn: "the Mobile Starter",
      workspaceName: "mobile-app",
      presetId: "1.bnekh.fem.ed",
      stackZh: "Expo App + NestJS API + Kustomize + Docker Hub + dotenv",
      stackEn: "Expo app + NestJS API + Kustomize + Docker Hub + dotenv",
    }),
    tags: ["expo", "react-native", "nestjs", "mobile"],
  },
  {
    id: "desktop-starter",
    category: "desktop",
    title: { zh: "桌面应用", en: "Desktop App" },
    tagline: {
      zh: "Electron + React + Vite + inversify，适合跨平台桌面工具",
      en: "Electron + React + Vite + inversify, for cross-platform desktop tools",
    },
    baseTemplates: ["electron-app"],
    presetId: "1.fea.ed",
    workspaceName: "desktop-app",
    cover: "/examples/desktop-starter/cover.png",
    prompt: makePrompt({
      titleZh: "桌面应用",
      titleEn: "the Desktop App starter",
      workspaceName: "desktop-app",
      presetId: "1.fea.ed",
      stackZh: "Electron App + dotenv",
      stackEn: "Electron app + dotenv",
    }),
    tags: ["electron", "react", "vite", "desktop"],
  },
  {
    id: "landing-starter",
    category: "web",
    title: { zh: "营销落地页", en: "Marketing Landing" },
    tagline: {
      zh: "Astro 静态站 + NestJS API 表单后端，适合需要表单 / 订阅的官网",
      en: "Astro static site + NestJS API for forms, ideal for marketing sites that need a backend",
    },
    baseTemplates: ["astro-site", "nestjs-api"],
    presetId: "1.bnekh.fasc.ed",
    workspaceName: "marketing-site",
    cover: "/examples/landing-starter/cover.png",
    prompt: makePrompt({
      titleZh: "营销落地页",
      titleEn: "the Marketing Landing starter",
      workspaceName: "marketing-site",
      presetId: "1.bnekh.fasc.ed",
      stackZh: "Astro 站点 + Cloudflare + NestJS API + Kustomize + Docker Hub + dotenv",
      stackEn: "Astro site + Cloudflare + NestJS API + Kustomize + Docker Hub + dotenv",
    }),
    tags: ["astro", "nestjs", "cloudflare", "marketing"],
  },
  {
    id: "docs-starter",
    category: "docs",
    title: { zh: "文档站", en: "Documentation Site" },
    tagline: {
      zh: "Astro Starlight，自带搜索 / 侧边栏 / 参考表，适合产品文档与开源项目",
      en: "Astro Starlight with built-in search, sidebar, and reference tables",
    },
    baseTemplates: ["starlight-docs"],
    presetId: "1.fslc.ed",
    workspaceName: "docs-site",
    cover: "/examples/docs-starter/cover.png",
    prompt: makePrompt({
      titleZh: "文档站",
      titleEn: "the Documentation Site starter",
      workspaceName: "docs-site",
      presetId: "1.fslc.ed",
      stackZh: "Starlight + Cloudflare + dotenv",
      stackEn: "Starlight + Cloudflare + dotenv",
    }),
    tags: ["starlight", "astro", "docs"],
  },
  {
    id: "consumer-starter",
    category: "consumer",
    title: { zh: "C 端 Web", en: "Consumer Web" },
    tagline: {
      zh: "Next.js App Router + SSR，部署到 Vercel，适合 SEO / 首屏要求高的 C 端内容站",
      en: "Next.js App Router + SSR on Vercel, for consumer sites where SEO and first paint matter",
    },
    baseTemplates: ["nextjs-app"],
    presetId: "1.fnav.ed",
    workspaceName: "consumer-web",
    cover: "/examples/consumer-starter/cover.png",
    prompt: makePrompt({
      titleZh: "C 端 Web",
      titleEn: "the Consumer Web starter",
      workspaceName: "consumer-web",
      presetId: "1.fnav.ed",
      stackZh: "Next.js + Vercel + dotenv",
      stackEn: "Next.js + Vercel + dotenv",
    }),
    tags: ["nextjs", "react", "vercel", "consumer"],
  },
  {
    id: "admin-starter",
    category: "admin",
    title: { zh: "后台管理", en: "Admin Dashboard" },
    tagline: {
      zh: "React + Vite CSR 配 NestJS API，适合 B 端管理后台与内部系统",
      en: "React + Vite CSR with NestJS API, for B2B admin and internal tooling",
    },
    baseTemplates: ["react-spa", "nestjs-api"],
    presetId: "1.bnekh.frsc.ed",
    workspaceName: "admin-dashboard",
    cover: "/examples/admin-starter/cover.png",
    prompt: makePrompt({
      titleZh: "后台管理",
      titleEn: "the Admin Dashboard starter",
      workspaceName: "admin-dashboard",
      presetId: "1.bnekh.frsc.ed",
      stackZh: "React SPA + Cloudflare + NestJS API + Kustomize + Docker Hub + dotenv",
      stackEn: "React SPA + Cloudflare + NestJS API + Kustomize + Docker Hub + dotenv",
    }),
    tags: ["react", "vite", "nestjs", "admin"],
  },
];
