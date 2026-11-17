import { useSearchParams } from "react-router-dom";
import { workspacePath } from "@/router/paths";
import { OpenWorkspace } from "@/lib/desktop/OpenWorkspace";
import { PageHeader } from "@/components/ui/page-layout";
import { AlertTriangle, FolderGit2, FolderPlus, RefreshCw, Search, Trash2, X } from "lucide-react";
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

const WorkspaceRow: React.FC<{ workspace: WorkspaceRegistryEntry; onForget(): void }> = ({
	workspace,
	onForget,
}) => {
	const { t, i18n } = useTranslation();
	const countUnavailable =
		(workspace.status === "missing" || workspace.status === "invalid") &&
		workspace.projectCount === 0;
	const ready = workspace.status === "ready";
	return (
		<article className="group relative min-w-0 border-b border-border last:border-b-0 hover:bg-muted/40">
			<EnvironmentLink
				to={workspacePath(workspace.entryId)}
				className="grid items-center gap-x-6 gap-y-3 p-4 pr-12 outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring @min-[640px]:grid-cols-[minmax(0,1fr)_10rem] @min-[960px]:grid-cols-[minmax(0,1fr)_10rem_13rem]"
			>
				<div className="flex min-w-0 items-center gap-3">
					<FolderGit2 aria-hidden className="size-5 shrink-0 text-muted-foreground" />
					<div className="flex min-w-0 flex-col gap-1">
						<h2 className="break-words text-sm font-medium">{workspace.name}</h2>
						<p title={workspace.root} className="truncate font-mono text-xs text-muted-foreground">
							{workspace.root}
						</p>
					</div>
				</div>
				<div className="flex flex-wrap items-center gap-3">
					<span className="whitespace-nowrap text-xs text-muted-foreground">
						<span className="font-mono text-sm text-foreground">
							{countUnavailable ? "—" : workspace.projectCount}
						</span>{" "}
						{t("workspaces.home.projects", { count: workspace.projectCount })}
					</span>
					<Badge
						variant={
							ready
								? "ghost"
								: workspace.status === "invalid"
									? "error"
									: workspace.status === "missing"
										? "muted"
										: "warning"
						}
						className={ready ? "gap-1.5 px-0 text-success-foreground" : "gap-1.5"}
					>
						{ready ? (
							<span aria-hidden className="size-1.5 rounded-full bg-current" />
						) : (
							<AlertTriangle />
						)}
						{t(`workspaces.status.${workspace.status}`)}
					</Badge>
				</div>
				<time
					dateTime={workspace.lastSeenAt}
					className="text-xs text-muted-foreground @min-[640px]:col-span-2 @min-[960px]:col-span-1 @min-[960px]:text-right"
				>
					{t("workspaces.home.lastDetected")} ·{" "}
					{formatLastSeen(workspace.lastSeenAt, i18n.resolvedLanguage ?? i18n.language)}
				</time>
			</EnvironmentLink>
			<Tooltip>
				<TooltipTrigger asChild>
					<Button
						variant="danger-ghost"
						size="icon"
						className="absolute right-3 top-4"
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
	const [searchParams, setSearchParams] = useSearchParams();
	const query = searchParams.get("q") ?? "";
	const filter = searchParams.get("status") === "attention" ? "attention" : "all";
	function setQuery(value: string) {
		setSearchParams(
			(current) => {
				const next = new URLSearchParams(current);
				if (value) next.set("q", value);
				else next.delete("q");
				return next;
			},
			{ replace: true },
		);
	}
	function setFilter(value: "all" | "attention") {
		setSearchParams(
			(current) => {
				const next = new URLSearchParams(current);
				if (value === "attention") next.set("status", value);
				else next.delete("status");
				return next;
			},
			{ replace: true },
		);
	}
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
		setSearchParams(
			(current) => {
				const next = new URLSearchParams(current);
				next.delete("q");
				next.delete("status");
				return next;
			},
			{ replace: true },
		);
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
			<div className="@container mx-auto flex w-full max-w-[1120px] flex-col gap-8 pb-4">
				<PageHeader
					title={t("workspaces.home.title")}
					description={t("workspaces.home.description")}
					actions={
						<>
							<OpenWorkspace />
							<Button
								variant="outline"
								onClick={() => void registry.mutate()}
								disabled={registry.isValidating}
								aria-busy={registry.isValidating}
							>
								{registry.isValidating ? <Spinner aria-hidden /> : <RefreshCw />}
								{t("workspaces.home.refresh")}
							</Button>
						</>
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
					<div role="status" className="flex flex-col gap-3">
						<span className="sr-only">{t("workspaces.home.loading")}</span>
						{[0, 1, 2].map((key) => (
							<Skeleton key={key} className="h-20 rounded-lg" />
						))}
					</div>
				) : registry.data ? (
					<>
						{workspaces.length === 0 ? (
							<Empty className="min-h-72">
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
								className="flex flex-col gap-4"
								aria-busy={registry.isValidating}
							>
								<div className="flex flex-col gap-3 ud-sm:flex-row ud-sm:items-center ud-sm:justify-between">
									<div
										className="flex flex-wrap items-center gap-2"
										role="group"
										aria-label={t("workspaces.home.filterLabel")}
									>
										<Button
											variant="ghost"
											className="aria-pressed:bg-muted aria-pressed:text-foreground aria-pressed:shadow-[inset_0_0_0_1px_var(--border)]"
											aria-pressed={filter === "all"}
											onClick={() => setFilter("all")}
										>
											{t("workspaces.home.all")}
											<span className="font-mono text-xs tabular-nums text-muted-foreground">
												{workspaces.length}
											</span>
										</Button>
										<Button
											variant="ghost"
											className="aria-pressed:bg-muted aria-pressed:text-foreground aria-pressed:shadow-[inset_0_0_0_1px_var(--border)]"
											aria-pressed={filter === "attention"}
											onClick={() => setFilter("attention")}
										>
											{t("workspaces.home.attention")}
											<span className="font-mono text-xs tabular-nums text-muted-foreground">
												{attention}
											</span>
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
									<div className="overflow-hidden rounded-lg border border-border bg-card">
										{filtered.map((workspace) => (
											<WorkspaceRow
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
									<Empty className="min-h-64">
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
