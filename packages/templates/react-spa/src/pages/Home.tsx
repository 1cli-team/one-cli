/**
 * Welcome screen — kept intentionally minimal for AI agents to extend.
 *
 * Pre-wired infrastructure (don't recreate; import from these paths):
 *   - Theme store:   src/lib/stores/theme.ts        (zustand, toggles `data-theme`)
 *   - Toast:         src/hooks/useToast.ts          (sonner-backed, see src/lib/toast.ts)
 *   - HTTP client:   src/lib/http.ts                (axios instance)
 *   - SWR provider:  src/providers/SWRProvider.tsx  (mounted in src/main.tsx)
 *   - Theme provider: src/providers/ThemeProvider.tsx
 *   - Error boundary: src/components/ErrorBoundary.tsx (wraps the router)
 *   - Demo API:      src/api/demo.ts                (getDemo, demoKey for SWR)
 *   - App metadata:  src/lib/app-info.ts            (appInfo, getEnvironmentLabel)
 *   - Design tokens: src/styles/tokens.css          (CSS variables, light + dark)
 *
 * Atomic design (advisory — physical folders NOT enforced):
 *   - atoms      → src/components/ui/        (shadcn primitives: Button, Card, Badge, Input, Alert)
 *   - molecules  → src/components/           (compose atoms; e.g. ResourceLink below)
 *   - organisms  → src/components/sections/  (page-level blocks; create when needed)
 *   - pages      → src/pages/                (this file lives here)
 *
 * Design tokens — DO use these utilities (they map to CSS vars in tokens.css):
 *   - Surface:    bg-background / bg-card / bg-popover
 *   - Text:       text-foreground / text-muted-foreground / text-primary
 *   - Border:     border-border / border-input
 *   - Accent:     bg-primary / bg-secondary / bg-destructive
 *   - Semantic:   bg-success-surface / bg-info-surface / bg-warning-surface / bg-error-surface
 *   DON'T hardcode hex / rgb in Tailwind classes.
 */

import { ArrowRight, BookOpen, Github } from "lucide-react";
import type React from "react";

export const Home: React.FC = () => (
	<section className="mx-auto flex max-w-2xl flex-col items-center gap-8 py-16 text-center">
		<div className="rounded-2xl bg-[#0a0a0a] px-6 py-4 shadow-lg shadow-black/10">
			<img src="/onecli-logo-inverted.svg" alt="One CLI" className="h-8 w-auto" />
		</div>

		<div className="space-y-3">
			<span className="inline-flex items-center rounded-full border border-border px-3 py-1 text-[11px] font-medium uppercase tracking-[0.18em] text-muted-foreground">
				React + Vite · SPA
			</span>
			<h1 className="text-4xl font-semibold tracking-tight text-foreground sm:text-5xl">
				Welcome to One CLI
			</h1>
			<p className="text-base text-muted-foreground sm:text-lg">
				一个由 One CLI 生成的 React SPA 脚手架，已预置 shadcn/ui、Tailwind v4、SWR 与设计令牌。
			</p>
		</div>

		<p className="text-sm text-muted-foreground">
			Edit{" "}
			<code className="rounded bg-muted px-2 py-1 font-mono text-xs text-foreground">
				src/pages/Home.tsx
			</code>{" "}
			and save to start.
		</p>

		<div className="flex flex-col items-center gap-3 sm:flex-row">
			<a
				href="https://1cli.dev/zh/docs/quick-start/"
				target="_blank"
				rel="noreferrer noopener"
				className="inline-flex items-center gap-2 rounded-full bg-primary px-5 py-2.5 text-sm font-medium text-primary-foreground transition-opacity hover:opacity-90"
			>
				<BookOpen className="size-4" />
				开始构建
				<ArrowRight className="size-4" />
			</a>
			<a
				href="https://github.com/torchstellar-team/one-cli"
				target="_blank"
				rel="noreferrer noopener"
				className="inline-flex items-center gap-2 rounded-full border border-border px-5 py-2.5 text-sm font-medium text-foreground transition-colors hover:bg-muted"
			>
				<Github className="size-4" />
				GitHub
			</a>
		</div>
	</section>
);
