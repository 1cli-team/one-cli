import { PageHeader } from "@/components/ui/page-layout";
import {
	AlertTriangle,
	ArrowUpRight,
	CheckCircle2,
	ChevronDown,
	FolderGit2,
	FolderPlus,
	RefreshCw,
	Search,
	Terminal,
	Trash2,
	X,
} from "lucide-react";
import type React from "react";
import { useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import useSWR from "swr";
import { forgetWorkspace, getWorkspaces, workspacesKey } from "@/api/workspaces";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import {
	AlertDialog,
	AlertDialogAction,
	AlertDialogCancel,
	AlertDialogContent,
	AlertDialogDescription,
	AlertDialogFooter,
	AlertDialogHeader,
	AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
	Empty,
	EmptyDescription,
	EmptyHeader,
	EmptyMedia,
	EmptyTitle,
} from "@/components/ui/empty";
import {
	InputGroup,
	InputGroupAddon,
	InputGroupButton,
	InputGroupInput,
} from "@/components/ui/input-group";
import { Skeleton } from "@/components/ui/skeleton";
import { Spinner } from "@/components/ui/spinner";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip";
import { EnvironmentLink } from "@/features/environment-context/EnvironmentLink";
import { useToast } from "@/hooks/useToast";
import type { WorkspaceRegistryEntry } from "@/types/api";

function formatLastSeen(value: string, locale: string): string {
	const date = new Date(value);
	if (Number.isNaN(date.getTime())) return value;
	return new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeStyle: "short" }).format(date);
}

const WorkspaceCard: React.FC<{ workspace: WorkspaceRegistryEntry; onForget(): void }> = ({
	workspace,
	onForget,
}) => {
	const { t, i18n } = useTranslation();
	const countUnavailable =
		(workspace.status === "missing" || workspace.status === "invalid") &&
		workspace.projectCount === 0;
	const ready = workspace.status === "ready";
	return (
		<article className="group relative min-w-0 rounded-lg border border-border bg-card transition-[border-color,box-shadow] duration-150 hover:border-primary/40 hover:shadow-sm">
			<EnvironmentLink
				to={`/workspace/${encodeURIComponent(workspace.entryId)}`}
				className="flex h-full flex-col rounded-lg p-5 outline-none focus-visible:ring-2 focus-visible:ring-ring"
			>
				<div className="flex min-w-0 items-center gap-3 pr-8">
					<span className="grid size-10 shrink-0 place-items-center rounded-lg bg-accent text-primary-text">
						<FolderGit2 className="size-5" />
					</span>
					<h2 className="min-w-0 break-words text-base font-semibold leading-6">
						{workspace.name}
					</h2>
				</div>
				<p
					title={workspace.root}
					className="mt-4 truncate font-mono text-xs leading-5 text-muted-foreground"
				>
					{workspace.root}
				</p>
				<div className="my-5 flex flex-wrap items-center justify-between gap-3">
					<div className="flex items-baseline gap-2">
						<span className="text-2xl font-semibold leading-9">
							{countUnavailable ? "-" : workspace.projectCount}
						</span>
						<span className="text-sm text-muted-foreground">{t("workspaces.home.projects")}</span>
					</div>
					<Badge
						variant={
							ready
								? "success"
								: workspace.status === "invalid"
									? "error"
									: workspace.status === "missing"
										? "muted"
										: "warning"
						}
					>
						{ready ? <CheckCircle2 /> : <AlertTriangle />}
						{t(`workspaces.status.${workspace.status}`)}
					</Badge>
				</div>
				<div className="mt-auto flex items-start justify-between gap-3 border-t border-border pt-3 text-xs leading-5 text-muted-foreground">
					<time dateTime={workspace.lastSeenAt}>
						{t("workspaces.home.lastDetected")} ·{" "}
						{formatLastSeen(workspace.lastSeenAt, i18n.resolvedLanguage ?? i18n.language)}
					</time>
					<ArrowUpRight aria-hidden className="mt-0.5 size-4 shrink-0 text-primary-text" />
				</div>
			</EnvironmentLink>
			<Tooltip>
				<TooltipTrigger asChild>
					<Button
						variant="danger-ghost"
						size="icon"
						className="absolute top-5 right-4"
						aria-label={t("workspaces.forget.action", { name: workspace.name })}
						onClick={onForget}
					>
						<Trash2 />
					</Button>
				</TooltipTrigger>
				<TooltipContent>{t("workspaces.forget.action", { name: workspace.name })}</TooltipContent>
			</Tooltip>
		</article>
	);
};

export const WorkspaceHome: React.FC = () => {
	const { t } = useTranslation();
	const toast = useToast();
	const registry = useSWR(workspacesKey, getWorkspaces);
	const [query, setQuery] = useState("");
	const [filter, setFilter] = useState<"all" | "attention">("all");
	const searchRef = useRef<HTMLInputElement>(null);
	const [workspaceToForget, setWorkspaceToForget] = useState<WorkspaceRegistryEntry | null>(null);
	const [forgetting, setForgetting] = useState(false);
	const [forgetError, setForgetError] = useState("");
	const workspaces = registry.data?.workspaces ?? [];
	const attention = workspaces.filter((workspace) => workspace.status !== "ready").length;
	const filtered = workspaces.filter(
		(workspace) =>
			(filter === "all" || workspace.status !== "ready") &&
			`${workspace.name} ${workspace.root} ${workspace.id ?? ""}`
				.toLocaleLowerCase()
				.includes(query.trim().toLocaleLowerCase()),
	);
	function clearFilters() {
		setQuery("");
		setFilter("all");
		searchRef.current?.focus();
	}
	async function confirmForget() {
		if (!workspaceToForget || forgetting) return;
		setForgetting(true);
		setForgetError("");
		try {
			await forgetWorkspace(workspaceToForget.entryId);
			await registry.mutate(
				(current) =>
					current
						? {
								...current,
								currentEntryId:
									current.currentEntryId === workspaceToForget.entryId
										? undefined
										: current.currentEntryId,
								workspaces: current.workspaces.filter(
									(entry) => entry.entryId !== workspaceToForget.entryId,
								),
							}
						: current,
				{ revalidate: false },
			);
			toast.success(t("workspaces.forget.done", { name: workspaceToForget.name }));
			setWorkspaceToForget(null);
		} catch (error) {
			setForgetError((error as { message?: string }).message || t("workspaces.forget.failed"));
		} finally {
			setForgetting(false);
		}
	}
	return (
		<TooltipProvider delayDuration={300}>
			<div className="mx-auto w-full max-w-6xl space-y-6 pb-4">
				<PageHeader
					title={t("workspaces.home.title")}
					description={t("workspaces.home.description")}
					actions={
						<Button
							variant="outline"
							onClick={() => void registry.mutate()}
							disabled={registry.isValidating}
							aria-busy={registry.isValidating}
						>
							{registry.isValidating ? <Spinner aria-hidden /> : <RefreshCw />}
							{t("workspaces.home.refresh")}
						</Button>
					}
				/>
				{registry.error ? (
					<Alert variant="destructive">
						<AlertTriangle />
						<AlertTitle>{t("workspaces.home.loadFailedTitle")}</AlertTitle>
						<AlertDescription>
							<p>
								{(registry.error as { message?: string }).message ??
									t("workspaces.home.loadFailedDescription")}
							</p>
							<Button
								variant="outline"
								className="mt-2"
								disabled={registry.isValidating}
								onClick={() => void registry.mutate()}
							>
								{t("workspaces.home.retry")}
							</Button>
						</AlertDescription>
					</Alert>
				) : null}
				{registry.isLoading && !registry.data ? (
					<div role="status" className="grid gap-4 ud-sm:grid-cols-2 ud-lg:grid-cols-3">
						<span className="sr-only">{t("workspaces.home.loading")}</span>
						{[0, 1, 2].map((key) => (
							<Skeleton key={key} className="h-56 rounded-lg" />
						))}
					</div>
				) : registry.data ? (
					<>
						<details className="group rounded-lg border border-border bg-card px-4 py-3">
							<summary className="flex cursor-pointer list-none items-center gap-3 rounded-md text-sm outline-none focus-visible:ring-2 focus-visible:ring-ring">
								<Terminal className="size-4 text-primary-text" aria-hidden="true" />
								<span className="flex-1">{t("workspaces.home.registrationHelp")}</span>
								<ChevronDown
									className="size-4 text-muted-foreground transition-transform group-open:rotate-180"
									aria-hidden="true"
								/>
							</summary>
							<p className="mt-3 text-sm text-muted-foreground">
								{t("workspaces.home.registrationHint")}
							</p>
						</details>
						{workspaces.length === 0 ? (
							<Empty className="min-h-72 rounded-lg border border-dashed border-border bg-card">
								<EmptyHeader>
									<EmptyMedia variant="icon">
										<FolderPlus className="text-primary-text" />
									</EmptyMedia>
									<EmptyTitle>
										<h2>{t("workspaces.home.emptyTitle")}</h2>
									</EmptyTitle>
									<EmptyDescription>{t("workspaces.home.emptyDescription")}</EmptyDescription>
								</EmptyHeader>
							</Empty>
						) : (
							<section
								aria-label={t("workspaces.home.listLabel")}
								className="space-y-4"
								aria-busy={registry.isValidating}
							>
								<div className="flex flex-col gap-3 ud-sm:flex-row ud-sm:items-center ud-sm:justify-between">
									<div
										className="flex flex-wrap items-center gap-2"
										role="group"
										aria-label={t("workspaces.home.filterLabel")}
									>
										<Button
											variant={filter === "all" ? "secondary" : "ghost"}
											aria-pressed={filter === "all"}
											onClick={() => setFilter("all")}
										>
											{t("workspaces.home.all")}
											<span className="tabular-nums">{workspaces.length}</span>
										</Button>
										<Button
											variant={filter === "attention" ? "secondary" : "ghost"}
											aria-pressed={filter === "attention"}
											onClick={() => setFilter("attention")}
										>
											{t("workspaces.home.attention")}
											<span className="tabular-nums">{attention}</span>
										</Button>
									</div>
									<InputGroup className="ud-sm:max-w-72">
										<InputGroupAddon>
											<Search />
										</InputGroupAddon>
										<InputGroupInput
											ref={searchRef}
											value={query}
											onChange={(event) => setQuery(event.target.value)}
											placeholder={t("workspaces.home.search")}
											aria-label={t("workspaces.home.search")}
										/>
										{query ? (
											<InputGroupAddon align="inline-end">
												<InputGroupButton
													size="icon-xs"
													aria-label={t("workspaces.home.clearSearch")}
													onClick={() => {
														setQuery("");
														searchRef.current?.focus();
													}}
												>
													<X />
												</InputGroupButton>
											</InputGroupAddon>
										) : null}
									</InputGroup>
								</div>
								<p role="status" className="sr-only">
									{t("workspaces.home.resultCount", { count: filtered.length })}
								</p>
								{filtered.length > 0 ? (
									<div className="grid grid-cols-1 gap-4 ud-sm:grid-cols-2 ud-lg:grid-cols-3 ud-xl:grid-cols-4">
										{filtered.map((workspace) => (
											<WorkspaceCard
												key={workspace.entryId}
												workspace={workspace}
												onForget={() => {
													setForgetError("");
													setWorkspaceToForget(workspace);
												}}
											/>
										))}
									</div>
								) : (
									<Empty className="min-h-64 rounded-lg border border-dashed border-border bg-card">
										<EmptyHeader>
											<EmptyMedia variant="icon">
												<Search />
											</EmptyMedia>
											<EmptyTitle>{t("workspaces.home.noResults")}</EmptyTitle>
											<EmptyDescription>
												{t("workspaces.home.noResultsDescription")}
											</EmptyDescription>
										</EmptyHeader>
										<Button variant="outline" onClick={clearFilters}>
											{t("workspaces.home.clearFilters")}
										</Button>
									</Empty>
								)}
							</section>
						)}
					</>
				) : null}
				<AlertDialog
					open={Boolean(workspaceToForget)}
					onOpenChange={(open) => !open && !forgetting && setWorkspaceToForget(null)}
				>
					<AlertDialogContent>
						<AlertDialogHeader>
							<AlertDialogTitle>
								{workspaceToForget
									? t("workspaces.forget.action", { name: workspaceToForget.name })
									: ""}
							</AlertDialogTitle>
							<AlertDialogDescription>
								{workspaceToForget
									? t("workspaces.forget.confirm", { name: workspaceToForget.name })
									: ""}
							</AlertDialogDescription>
						</AlertDialogHeader>
						{forgetError ? (
							<p role="alert" className="text-sm text-error-foreground">
								{forgetError}
							</p>
						) : null}
						<AlertDialogFooter>
							<AlertDialogCancel disabled={forgetting}>{t("form.cancel")}</AlertDialogCancel>
							<AlertDialogAction
								variant="destructive"
								disabled={forgetting}
								onClick={(event) => {
									event.preventDefault();
									void confirmForget();
								}}
							>
								{forgetting ? <Spinner /> : <Trash2 />}
								{workspaceToForget
									? t("workspaces.forget.action", { name: workspaceToForget.name })
									: ""}
							</AlertDialogAction>
						</AlertDialogFooter>
					</AlertDialogContent>
				</AlertDialog>
			</div>
		</TooltipProvider>
	);
};
