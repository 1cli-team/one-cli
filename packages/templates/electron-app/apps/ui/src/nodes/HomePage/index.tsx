/**
 * Welcome screen — kept intentionally minimal for AI agents to extend.
 *
 * Pre-wired infrastructure (don't recreate; import from these paths):
 *   - IPC client:       useElectron() in src/hooks/electron.ts
 *                       (electron.invoke / .send / .on; types from @app/preload/channels)
 *   - IPC channels:     IPC.dialog.open / IPC.shell.open / IPC.contextMenu.show, etc.
 *                       (full list in @app/preload/channels — DO NOT redefine)
 *   - IPC events:       EVENT.update.downloadProgress, etc. — subscribe via electron.on()
 *   - Store:            useAppStore (zustand + immer) in src/store/app-store.ts
 *   - HTTP client:      src/lib/http.ts (axios fetcher, e.g. for SWR with public APIs)
 *   - Toast:            sonner via src/components/ui/sonner.tsx (mounted by parent shell)
 *   - Routing:          react-router; routes defined alongside App.tsx
 *   - Design tokens:    src/styles/globals.css (CSS vars, light + dark via shadcn convention)
 *
 * Atomic design (advisory — physical folders NOT enforced):
 *   - atoms      → src/components/ui/      (shadcn primitives)
 *   - molecules  → src/components/         (compose atoms)
 *   - organisms  → src/nodes/<Page>/       (page-level blocks live here, alongside the page)
 *   - pages      → src/nodes/<Page>/index.tsx
 *
 * Design tokens — DO use these utilities (they map to CSS vars in globals.css):
 *   - Surface:    bg-background / bg-card / bg-popover
 *   - Text:       text-foreground / text-muted-foreground / text-primary
 *   - Border:     border-border / border-input
 *   - Accent:     bg-primary / bg-secondary / bg-destructive
 *   DON'T hardcode hex / rgb in Tailwind classes.
 */

import { ArrowRight, BookOpen, Github } from "lucide-react";
import type { FC } from "react";

const HomePage: FC = () => (
  <section className="mx-auto flex max-w-2xl flex-col items-center gap-8 py-16 text-center">
    <div className="rounded-2xl bg-[#0a0a0a] px-6 py-4 shadow-lg shadow-black/10">
      <img src="/onecli-logo-inverted.svg" alt="One CLI" className="h-8 w-auto" />
    </div>

    <div className="space-y-3">
      <span className="border-border text-muted-foreground inline-flex items-center rounded-full border px-3 py-1 text-[11px] font-medium tracking-[0.18em] uppercase">
        Electron + React · Desktop
      </span>
      <h1 className="text-foreground text-4xl font-semibold tracking-tight sm:text-5xl">
        Welcome to One CLI
      </h1>
      <p className="text-muted-foreground text-base sm:text-lg">
        一个由 One CLI 生成的 Electron 桌面端脚手架，已预置 shadcn/ui、Tailwind v4、IPC 桥与状态管理。
      </p>
    </div>

    <p className="text-muted-foreground text-sm">
      Edit{" "}
      <code className="bg-muted text-foreground rounded px-2 py-1 font-mono text-xs">
        apps/ui/src/nodes/HomePage/index.tsx
      </code>{" "}
      and save to start.
    </p>

    <div className="flex flex-col items-center gap-3 sm:flex-row">
      <a
        href="https://1cli.dev/zh/docs/quick-start/"
        target="_blank"
        rel="noreferrer noopener"
        className="bg-primary text-primary-foreground inline-flex items-center gap-2 rounded-full px-5 py-2.5 text-sm font-medium transition-opacity hover:opacity-90"
      >
        <BookOpen className="size-4" />
        开始构建
        <ArrowRight className="size-4" />
      </a>
      <a
        href="https://github.com/torchstellar-team/one-cli"
        target="_blank"
        rel="noreferrer noopener"
        className="border-border text-foreground hover:bg-muted inline-flex items-center gap-2 rounded-full border px-5 py-2.5 text-sm font-medium transition-colors"
      >
        <Github className="size-4" />
        GitHub
      </a>
    </div>
  </section>
);

export default HomePage;
