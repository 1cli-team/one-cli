import { useId, type ReactNode } from "react";
import { Card, CardContent } from "@/components/ui/card";
import { PageContent, PageHeader } from "@/components/ui/page-layout";

export function SettingsPage({
	title,
	children,
	status,
}: {
	title: string;
	children: ReactNode;
	status?: ReactNode;
}) {
	return (
		<PageContent>
			<PageHeader title={title} status={status} />
			<div className="@container flex w-full max-w-3xl flex-col gap-6">{children}</div>
		</PageContent>
	);
}

export function SettingsSection({ title, children }: { title?: string; children: ReactNode }) {
	const titleId = useId();
	return (
		<section className="flex min-w-0 flex-col gap-3" aria-labelledby={title ? titleId : undefined}>
			{title && (
				<h2 id={titleId} className="text-base font-semibold leading-6">
					{title}
				</h2>
			)}
			<Card>
				<CardContent className="flex flex-col gap-4 p-5">{children}</CardContent>
			</Card>
		</section>
	);
}

export function SettingsDetails({ children }: { children: ReactNode }) {
	return (
		<dl className="grid gap-x-6 gap-y-3 text-sm @min-[36rem]:grid-cols-[8rem_minmax(0,1fr)] [&>dt]:text-muted-foreground [&>dd]:min-w-0 [&>dd]:wrap-anywhere">
			{children}
		</dl>
	);
}

export function SettingsRow({ title, children }: { title: string; children: ReactNode }) {
	return (
		<div className="grid items-center gap-3 border-b border-border py-4 first:pt-0 last:border-b-0 last:pb-0 @min-[36rem]:grid-cols-[minmax(0,1fr)_14rem]">
			<h3 className="min-w-0 text-sm font-medium">{title}</h3>
			<div className="min-w-0 [&>button]:w-full [&>button]:justify-between">{children}</div>
		</div>
	);
}
