import type { LucideIcon } from "lucide-react";
import type { ReactNode } from "react";
import { EnvironmentLink } from "@/features/environment-context/EnvironmentLink";
import { cn } from "@/lib/utils";

export function ProjectCardGrid({ children }: { children: ReactNode }) {
	return (
		<div className="grid gap-3 @min-[36rem]:grid-cols-2 @min-[56rem]:grid-cols-3">{children}</div>
	);
}

export function ProjectCard({
	to,
	icon: Icon,
	title,
	summary,
	action,
}: {
	to: string;
	icon: LucideIcon;
	title: string;
	summary: ReactNode;
	action?: ReactNode;
}) {
	return (
		<article className="group/card relative min-w-0 transition-[translate] duration-150 ease-ui motion-safe:hover:-translate-y-0.5">
			<EnvironmentLink
				to={to}
				className="flex h-full min-w-0 items-start gap-3 rounded-lg border border-border bg-card p-4 transition-[border-color,box-shadow] duration-150 ease-ui group-hover/card:border-input group-hover/card:shadow-sm focus-visible:outline-ring"
			>
				<span className="grid size-9 shrink-0 place-items-center rounded-md border border-border bg-muted/50 text-muted-foreground">
					<Icon aria-hidden="true" className="size-4" />
				</span>
				<div className="min-w-0 flex-1">
					<h2 className={cn("break-words font-semibold", action && "pr-8")}>{title}</h2>
					<div className="text-xs text-muted-foreground">{summary}</div>
				</div>
			</EnvironmentLink>
			{action && <div className="absolute top-3 right-3">{action}</div>}
		</article>
	);
}
