import type { ReactNode } from "react";
import { Card, CardContent } from "@/components/ui/card";
import { cn } from "@/lib/utils";

export function SettingsPage({
	title,
	children,
	className,
}: {
	title: string;
	children: ReactNode;
	className?: string;
}) {
	return (
		<div className={cn("@container mx-auto flex w-full max-w-[1120px] flex-col gap-8", className)}>
			<h1 className="sr-only">{title}</h1>
			{children}
		</div>
	);
}

export function SettingsSection({
	title,
	actions,
	children,
}: {
	title: string;
	actions?: ReactNode;
	children: ReactNode;
}) {
	return (
		<section className="flex min-w-0 flex-col gap-3" aria-label={title}>
			<header className="flex flex-wrap items-center justify-between gap-3">
				<h2 className="text-base font-medium leading-6">{title}</h2>
				{actions}
			</header>
			<Card>
				<CardContent className="flex flex-col gap-4 p-5">{children}</CardContent>
			</Card>
		</section>
	);
}

export function SettingsDetails({ children }: { children: ReactNode }) {
	return (
		<dl className="grid gap-x-6 gap-y-3 text-sm @min-[720px]:grid-cols-[8rem_minmax(0,1fr)] [&>dt]:text-muted-foreground [&>dd]:min-w-0 [&>dd]:break-all">
			{children}
		</dl>
	);
}

export function SettingsRow({ title, children }: { title: string; children: ReactNode }) {
	return (
		<div className="grid items-center gap-4 border-b border-border py-4 first:pt-0 last:border-b-0 last:pb-0 @min-[960px]:grid-cols-[minmax(0,1fr)_14rem]">
			<h3 className="min-w-0 text-sm font-medium">{title}</h3>
			<div className="min-w-0 [&>button]:w-full [&>button]:justify-between">{children}</div>
		</div>
	);
}
