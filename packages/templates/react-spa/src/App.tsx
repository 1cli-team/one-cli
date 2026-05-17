import { MoonStar, SunMedium } from "lucide-react";
import type React from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { useThemeStore } from "@/lib/stores/theme";
import { AppRoutes } from "@/router/routes";

export const App: React.FC = () => {
	const { mode, toggle } = useThemeStore();

	return (
		<div className="min-h-screen bg-background text-foreground">
			<header className="sticky top-0 z-30 border-b border-border/70 bg-background/82 backdrop-blur-xl">
				<div className="mx-auto flex max-w-7xl items-center justify-between gap-4 px-6 py-4">
					<div className="flex min-w-0 items-center gap-4">
						<div className="flex h-11 w-11 items-center justify-center rounded-2xl bg-primary text-primary-foreground shadow-lg shadow-primary/20">
							<span className="text-base font-semibold">CSR</span>
						</div>
						<div className="min-w-0">
							<p className="truncate text-sm font-semibold tracking-[0.18em] text-muted-foreground uppercase">
								web-csr-react
							</p>
							<p className="truncate text-base font-medium text-foreground">
								React 19 + shadcn/ui starter
							</p>
						</div>
					</div>

					<div className="flex items-center gap-3">
						<Badge variant="secondary" className="hidden sm:inline-flex">
							{mode === "light" ? "Light mode" : "Dark mode"}
						</Badge>
						<Button onClick={toggle} variant="outline" size="sm">
							{mode === "light" ? <MoonStar /> : <SunMedium />}
							<span>{mode === "light" ? "切到深色" : "切到浅色"}</span>
						</Button>
					</div>
				</div>
			</header>

			<main className="mx-auto max-w-7xl px-6 py-8 sm:py-10">
				<AppRoutes />
			</main>

			<footer className="border-t border-border/60 bg-background/75">
				<div className="mx-auto flex max-w-7xl flex-col gap-2 px-6 py-6 text-sm text-muted-foreground sm:flex-row sm:items-center sm:justify-between">
					<p>CSR WebApp Template powered by shadcn/ui, Tailwind CSS v4 and Vite.</p>
					<p>© {new Date().getFullYear()} Template</p>
				</div>
			</footer>
		</div>
	);
};
