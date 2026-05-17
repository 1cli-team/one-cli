"use client";

import {
  Check,
  Copy,
  FileText,
  Layers3,
  MousePointer2,
  Puzzle,
} from "lucide-react";
import { useState } from "react";
import type { Locale } from "@/i18n";

const previewCopy = {
  zh: {
    label: "模板页入口",
    path: "/zh/templates/",
    direct: {
      eyebrow: "方式一",
      title: "直接选现成模板",
      body: "模板页已经按手机 App、桌面工具、官网、后台、文档站整理好。选一个示例，就能复制对应的 One CLI 创建命令。",
      points: ["按产品类型浏览", "打开详情看适用场景", "复制命令后执行"],
    },
    custom: {
      eyebrow: "方式二",
      title: "自定义组合",
      body: "没有刚好合适的示例时，在模板页点“自定义模板”，按需要组合网站、后台、文档、App 和共享代码。",
      points: ["自由组合多个部分", "自动生成 One CLI 命令", "适合已有明确想法"],
    },
    footer: "进入模板页后，先看现成示例；没有合适的，再打开自定义模板生成命令。",
  },
  en: {
    label: "Template page entry",
    path: "/en/templates/",
    direct: {
      eyebrow: "Option one",
      title: "Choose a ready example",
      body: "The templates page is organized by mobile app, desktop tool, website, admin, and docs. Pick an example, then copy the matching One CLI create command.",
      points: ["Browse by product type", "Open details for fit", "Copy and run the command"],
    },
    custom: {
      eyebrow: "Option two",
      title: "Build your own mix",
      body: "If no example fits, use “Build your own” on the templates page to combine website, backend, docs, app, and shared code.",
      points: ["Mix several parts", "Generate a One CLI command", "Use it when the idea is specific"],
    },
    footer: "On the templates page, start with ready examples; if none fits, open the custom builder to generate a command.",
  },
} as const;

export function HomeCopyButton({
  value,
  label = "Copy",
  copiedLabel = "Copied",
  className = "",
}: {
  value: string;
  label?: string;
  copiedLabel?: string;
  className?: string;
}) {
  const [copied, setCopied] = useState(false);

  async function handleCopy() {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1400);
    } catch {
      setCopied(false);
    }
  }

  return (
    <button
      type="button"
      onClick={handleCopy}
      className={`inline-flex items-center justify-center gap-1.5 rounded-md border border-white/10 px-2.5 py-1.5 text-xs font-medium text-stone-200 transition hover:border-orange-500/50 hover:text-white focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-orange-500 ${className}`}
      aria-label={copied ? copiedLabel : label}
      data-copied={copied}
    >
      {copied ? <Check className="size-3.5" /> : <Copy className="size-3.5" />}
      {copied ? copiedLabel : label}
    </button>
  );
}

export function HomeTemplatePreview({ lang = "zh" }: { lang?: Locale }) {
  const text = previewCopy[lang];

  return (
    <div className="overflow-hidden rounded-lg border border-[#292524] bg-[#1c1917]">
      <div className="border-b border-[#292524] p-4">
        <div className="min-w-0">
          <div className="flex items-center gap-2 text-xs font-semibold text-[#a8a29e]">
            <Layers3 className="size-4 text-orange-400" />
            {text.label}
          </div>
          <div className="mt-2 flex min-w-0 items-center gap-2 rounded-md border border-white/10 bg-black/20 px-3 py-2 font-mono text-xs text-stone-300">
            <span className="text-orange-400">1cli.dev</span>
            <span className="min-w-0 truncate">{text.path}</span>
          </div>
        </div>
      </div>

      <div className="grid gap-0 lg:grid-cols-2">
        <TemplateEntryCard
          icon={MousePointer2}
          eyebrow={text.direct.eyebrow}
          title={text.direct.title}
          body={text.direct.body}
          points={[...text.direct.points]}
          featured
        />
        <TemplateEntryCard
          icon={Puzzle}
          eyebrow={text.custom.eyebrow}
          title={text.custom.title}
          body={text.custom.body}
          points={[...text.custom.points]}
        />
      </div>

      <div className="flex items-start gap-3 border-t border-[#292524] bg-[#11100f] px-4 py-4">
        <span className="mt-0.5 inline-flex size-7 shrink-0 items-center justify-center rounded-md border border-orange-500/30 bg-orange-500/10 text-orange-300">
          <FileText className="size-4" />
        </span>
        <p className="text-sm leading-6 text-stone-400">{text.footer}</p>
      </div>
    </div>
  );
}

function TemplateEntryCard({
  icon: Icon,
  eyebrow,
  title,
  body,
  points,
  featured,
}: {
  icon: typeof MousePointer2;
  eyebrow: string;
  title: string;
  body: string;
  points: string[];
  featured?: boolean;
}) {
  return (
    <div
      className={[
        "flex min-h-[280px] flex-col justify-between border-[#292524] p-5 md:p-6",
        featured ? "border-b lg:border-b-0 lg:border-r" : "",
      ].join(" ")}
    >
      <div>
        <div className="flex items-center justify-between gap-3">
          <span className="inline-flex items-center gap-2 rounded-full border border-white/10 bg-black/20 px-3 py-1 text-xs font-medium text-stone-300">
            <Icon className="size-3.5 text-orange-400" />
            {eyebrow}
          </span>
        </div>
        <h3 className="mt-5 text-2xl font-bold leading-tight text-white">
          {title}
        </h3>
        <p className="mt-3 text-sm leading-6 text-stone-400">{body}</p>
        <div className="mt-5 grid gap-2">
          {points.map((point) => (
            <div key={point} className="flex items-center gap-2 text-sm text-stone-300">
              <Check className="size-4 shrink-0 text-orange-400" />
              {point}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
