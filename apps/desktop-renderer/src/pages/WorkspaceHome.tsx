import { workspaceGroupsKey } from "@/api/workspace-groups";
import { useSessionSWRConfig } from "@/lib/desktop/cache";
import { useSearchParams } from "react-router-dom";
import { workspacePath } from "@/router/paths";
import { OpenWorkspace } from "@/lib/desktop/OpenWorkspace";
import { ProjectCard, ProjectCardGrid } from "@/components/ProjectCards";
import { PageContent, PageHeader, PageToolbar, StatePanel } from "@/components/ui/page-layout";
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
	InputGroup,
	InputGroupAddon,
	InputGroupButton,
	InputGroupInput,
} from "@/components/ui/input-group";
import { Spinner } from "@/components/ui/spinner";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip";
import {
	isGitRepositoryWithoutManifest,
	workspaceDisplayStatus,
} from "@/features/workspace-navigation/workspace-status";
import { useToast } from "@/hooks/useToast";
import type { WorkspaceRegistryEntry } from "@/types/api";

const WorkspaceCard: React.FC<{ workspace: WorkspaceRegistryEntry; onForget(): void }> = ({
	workspace,
	onForget,
}) => {
	const { t } = useTranslation();
	const countUnavailable =
		(workspace.status === "missing" || workspace.status === "invalid") &&
		workspace.projectCount === 0;
	const status = workspaceDisplayStatus(workspace);
	const forgetLabel = t("workspaces.forget.action", { name: workspace.name });
	return (
		<ProjectCard
			to={workspacePath(workspace.entryId)}
			icon={FolderGit2}
			title={workspace.name}
			summary={
				<div className="flex flex-wrap items-center gap-x-2 gap-y-1">
					<span>
						{isGitRepositoryWithoutManifest(workspace)
							? t("git.repository")
							: countUnavailable
								? `— ${t("workspaces.home.projects", { count: 0 })}`
								: t("workspaces.home.projectCount", { count: workspace.projectCount })}
					</span>
					{status !== "ready" && (
						<Badge
							variant={status === "invalid" ? "error" : status === "missing" ? "muted" : "warning"}
							className="gap-1.5"
						>
							<AlertTriangle />
							{t(`workspaces.status.${status}`)}
						</Badge>
					)}
				</div>
			}
			action={
				<Tooltip>
					<TooltipTrigger asChild>
						<Button variant="danger-ghost" size="icon" aria-label={forgetLabel} onClick={onForget}>
							<Trash2 />
						</Button>
					</TooltipTrigger>
					<TooltipContent>{forgetLabel}</TooltipContent>
				</Tooltip>
			}
		/>
	);
};

export const WorkspaceHome: React.FC = () => {
	const { t } = useTranslation();
	const toast = useToast();
	const registry = useSWR(workspacesKey, getWorkspaces);
	const { mutate } = useSessionSWRConfig();
	const [searchParams, setSearchParams] = useSearchParams();
	const query = searchParams.get("q") ?? "";
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
	const searchRef = useRef<HTMLInputElement>(null);
	const [workspaceToForget, setWorkspaceToForget] = useState<WorkspaceRegistryEntry | null>(null);
	const [forgetting, setForgetting] = useState(false);
	const [forgetError, setForgetError] = useState("");
	const workspaces = registry.data?.workspaces ?? [];
	const search = query.trim().toLocaleLowerCase();
	const filtered = workspaces.filter((workspace) =>
		`${workspace.name} ${workspace.root} ${workspace.id ?? ""}`
			.toLocaleLowerCase()
			.includes(search),
	);
	function clearSearch() {
		setQuery("");
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
			await mutate(workspaceGroupsKey, undefined, { revalidate: true, throwOnError: false });
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
			<PageContent>
				<PageHeader
					title={t("workspaces.home.title")}
					actions={
						<>
							<Button
								variant="outline"
								onClick={() => void registry.mutate()}
								disabled={registry.isValidating}
								aria-busy={registry.isValidating}
							>
								{registry.isValidating ? <Spinner aria-hidden /> : <RefreshCw />}
								{t("workspaces.home.refresh")}
							</Button>
							<OpenWorkspace />
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
					<div role="status" className="flex items-center gap-2 py-1 text-sm text-muted-foreground">
						<Spinner aria-hidden="true" className="shrink-0 motion-reduce:animate-none" />
						<span>{t("workspaces.home.loading")}</span>
					</div>
				) : registry.data ? (
					<>
						{workspaces.length === 0 ? (
							<StatePanel
								framed
								icon={FolderPlus}
								title={t("workspaces.home.emptyTitle")}
								description={t("workspaces.home.emptyDescription")}
							/>
						) : (
							<section
								aria-label={t("workspaces.home.listLabel")}
								className="flex flex-col gap-4"
								aria-busy={registry.isValidating}
							>
								<PageToolbar>
									<InputGroup className="max-w-sm">
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
													onClick={clearSearch}
												>
													<X />
												</InputGroupButton>
											</InputGroupAddon>
										) : null}
									</InputGroup>
								</PageToolbar>
								<p role="status" className="sr-only">
									{t("workspaces.home.resultCount", { count: filtered.length })}
								</p>
								{filtered.length > 0 ? (
									<ProjectCardGrid>
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
									</ProjectCardGrid>
								) : (
									<StatePanel
										framed
										icon={Search}
										title={t("workspaces.home.noResults")}
										description={t("workspaces.home.noResultsDescription")}
									>
										<Button variant="outline" onClick={clearSearch}>
											{t("workspaces.home.clearSearch")}
										</Button>
									</StatePanel>
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
			</PageContent>
		</TooltipProvider>
	);
};
