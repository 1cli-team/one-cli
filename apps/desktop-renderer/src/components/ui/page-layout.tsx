import type { ComponentProps, ReactNode } from "react";
import { AlertCircle, Search, type LucideIcon } from "lucide-react";
import { Tabs } from "radix-ui";
import { cn } from "@/lib/utils";
import { InputGroup, InputGroupAddon, InputGroupInput } from "@/components/ui/input-group";

export function PageContent({
	children,
	scroll,
	className,
}: {
	children: ReactNode;
	scroll?: boolean;
	className?: string;
}) {
	const content = (
		<div className={cn("@container mx-auto flex w-full max-w-[1120px] flex-col gap-4", className)}>
			{children}
		</div>
	);
	return scroll ? <div className="h-full overflow-y-auto p-4 ud-md:p-6">{content}</div> : content;
}

export function PageTabList({ className, ...props }: ComponentProps<typeof Tabs.List>) {
	return (
		<Tabs.List
			className={cn("flex gap-1 overflow-x-auto border-b border-border", className)}
			{...props}
		/>
	);
}

export function PageTab({ className, ...props }: ComponentProps<typeof Tabs.Trigger>) {
	return (
		<Tabs.Trigger
			className={cn(
				"min-h-9 shrink-0 border-b-2 border-transparent px-3 text-sm font-medium text-muted-foreground outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring data-[state=active]:border-primary data-[state=active]:text-primary-text",
				className,
			)}
			{...props}
		/>
	);
}

export function PageToolbar({ className, ...props }: ComponentProps<"div">) {
	return (
		<div
			className={cn(
				"flex min-w-0 flex-wrap items-center gap-3 rounded-lg border border-border bg-card p-3",
				className,
			)}
			{...props}
		/>
	);
}

export function PageHeader({
	title,
	description,
	actions,
	status,
}: {
	title: string;
	description?: ReactNode;
	actions?: ReactNode;
	status?: ReactNode;
}) {
	return (
		<header className="flex shrink-0 flex-wrap items-center justify-between gap-x-6 gap-y-3">
			<div className="flex min-w-0 flex-col gap-1">
				<div className="flex flex-wrap items-center gap-x-3 gap-y-1">
					<h1 className="text-[1.75rem] leading-9 font-semibold tracking-[-0.018em]">{title}</h1>
					{status}
				</div>
				{description && (
					<p className="max-w-3xl text-sm leading-5 text-muted-foreground">{description}</p>
				)}
			</div>
			{actions && <div className="flex max-w-full flex-wrap items-center gap-2">{actions}</div>}
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
					<h2 className="text-base font-semibold">{title}</h2>
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
	framed,
}: {
	icon: LucideIcon;
	heading?: "h1" | "h2";
	title: string;
	description?: ReactNode;
	children?: ReactNode;
	className?: string;
	framed?: boolean;
}) {
	return (
		<div
			className={cn(
				"flex min-h-48 flex-col items-center justify-center px-6 py-8 text-center",
				framed && "rounded-lg border border-border bg-card",
				className,
			)}
		>
			<span className="mb-3 grid size-9 place-items-center rounded-md border border-border bg-card text-muted-foreground">
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

export function SearchInput({
	groupClassName,
	...props
}: ComponentProps<typeof InputGroupInput> & { groupClassName?: string }) {
	return (
		<InputGroup className={groupClassName}>
			<InputGroupAddon>
				<Search aria-hidden="true" />
			</InputGroupAddon>
			<InputGroupInput {...props} />
		</InputGroup>
	);
}
