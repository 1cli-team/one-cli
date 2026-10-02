"use client";

import { useEffect } from "react";
import { ThemeToggle } from "@/components/theme-toggle";
import { Button } from "@/components/ui/button";
import { useUI } from "@/stores/ui";

export default function HomePage() {
	const locale = useUI((state) => state.locale);
	const toggleLocale = useUI((state) => state.toggleLocale);
	useEffect(() => {
		document.documentElement.lang = locale === "zh" ? "zh-CN" : "en-US";
	}, [locale]);
	return (
		<main className="mx-auto max-w-2xl space-y-6 px-6 py-16">
			<header className="flex items-center justify-between">
				<span>One CLI · Next.js</span>
				<div className="flex gap-2">
					<Button variant="outline" onClick={toggleLocale} aria-label="切换语言 / Change language">
						{locale === "en" ? "中文" : "English"}
					</Button>
					<ThemeToggle />
				</div>
			</header>
			<h1 className="text-3xl font-semibold">{locale === "en" ? "Welcome" : "欢迎"}</h1>
			<p className="text-muted-foreground">
				{locale === "en" ? "Start building your application here." : "从这里开始构建你的应用。"}
			</p>
			<a className="text-primary underline" href="https://1cli.dev">
				{locale === "en" ? "Documentation" : "文档"}
			</a>
		</main>
	);
}
