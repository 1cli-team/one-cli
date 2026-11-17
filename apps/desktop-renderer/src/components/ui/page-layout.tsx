import type { ComponentProps, ReactNode } from "react";
import { AlertCircle, Search, type LucideIcon } from "lucide-react";
import { cn } from "@/lib/utils";
import { InputGroup, InputGroupAddon, InputGroupInput } from "@/components/ui/input-group";

export function PageHeader({
	title,
	description,
	actions,
}: {
	title: string;
	description?: string;
	actions?: ReactNode;
}) {
	return (
		<header className="flex shrink-0 flex-wrap items-start justify-between gap-4">
			<div className="flex min-w-0 flex-col gap-2">
				<h1 className="text-2xl font-medium leading-8">{title}</h1>
				{description && (
					<p className="max-w-3xl text-sm leading-5 text-muted-foreground">{description}</p>
				)}
			</div>
			{actions && <div className="flex shrink-0 flex-wrap items-center gap-2">{actions}</div>}
		</header>
	);
}

export function SectionHeading({
	icon: Icon,
	title,
	description,
	actions,
}: {
	icon: LucideIcon;
	title: string;
	description?: string;
	actions?: ReactNode;
}) {
	return (
		<div className="flex flex-wrap items-start justify-between gap-3">
			<div className="flex min-w-0 items-start gap-3">
				<span className="grid size-6 shrink-0 place-items-center text-muted-foreground">
					<Icon className="size-4" aria-hidden="true" />
				</span>
				<div className="min-w-0">
					<h2 className="text-base font-medium">{title}</h2>
					{description && <p className="mt-1 text-sm text-muted-foreground">{description}</p>}
				</div>
			</div>
			{actions}
		</div>
	);
}

export function ErrorNotice({ children, action }: { children: ReactNode; action?: ReactNode }) {
	return (
		<div
			role="alert"
			className="flex flex-wrap items-start gap-3 rounded-md border border-error-border bg-error-surface p-3 text-sm text-error-foreground"
		>
			<AlertCircle className="mt-0.5 size-4 shrink-0" aria-hidden="true" />
			<div className="min-w-0 flex-1 break-words">{children}</div>
			{action}
		</div>
	);
}

export function StatePanel({
	icon: Icon,
	heading: Heading = "h2",
	title,
	description,
	children,
	className,
}: {
	icon: LucideIcon;
	heading?: "h1" | "h2";
	title: string;
	description?: ReactNode;
	children?: ReactNode;
	className?: string;
}) {
	return (
		<div
			className={cn(
				"flex min-h-60 flex-col items-center justify-center px-6 py-10 text-center",
				className,
			)}
		>
			<span className="mb-4 grid size-10 place-items-center rounded-lg bg-muted text-muted-foreground">
				<Icon className="size-5" aria-hidden="true" />
			</span>
			<Heading className="text-base font-medium">{title}</Heading>
			{description && (
				<div className="mt-2 max-w-lg text-sm text-muted-foreground">{description}</div>
			)}
			{children && <div className="mt-5 flex flex-wrap justify-center gap-2">{children}</div>}
		</div>
	);
}

export function SearchInput(props: ComponentProps<typeof InputGroupInput>) {
	return (
		<InputGroup>
			<InputGroupAddon>
				<Search aria-hidden="true" />
			</InputGroupAddon>
			<InputGroupInput {...props} />
		</InputGroup>
	);
}
