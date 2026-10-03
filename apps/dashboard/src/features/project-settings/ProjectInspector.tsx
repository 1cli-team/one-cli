import {
	manifestDraftKey,
	useManifestDraftStore,
} from "@/features/manifest-draft/manifest-draft-store";
import {
	Plus,
	Server,
	AppWindow,
	Package,
	LockKeyhole,
	KeyRound,
	ChevronRight,
	type LucideIcon,
} from "lucide-react";
import { Collapsible } from "radix-ui";
import type React from "react";
import { useEffect, useId, useState } from "react";
import { useTranslation } from "react-i18next";
import { useSearchParams } from "react-router-dom";
import useSWR from "swr";
import {
	getProjectSettings,
	getWorkspaceEnvironment,
	projectSettingsKey,
	workspaceEnvironmentKey,
} from "@/api/workspace";
import { CreateProjectDialog } from "@/features/project-creation/CreateProjectDialog";
import { ManifestSaveControl } from "@/features/manifest-draft/ManifestSaveControl";
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
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { EnvironmentSelector } from "@/features/environment-context/EnvironmentSelector";
import { useEnvironmentDirtyStore } from "@/features/environment-context/environment-dirty-store";
import { EnvironmentForm } from "@/features/project-settings/forms/EnvironmentForm";
import { GeneralForm } from "@/features/project-settings/forms/GeneralForm";
import { InfisicalBindingDialog } from "@/features/infisical-binding/InfisicalBindingDialog";
import { cn } from "@/lib/utils";
import type { OverviewProject } from "@/types/api";

interface ProjectInspectorProps {
	environments?: string[];
	projects: OverviewProject[];
	environment: string;
	workspaceEntryId?: string;
	readOnly?: boolean;
}

const PROJECT_GROUPS = [
	{ kind: "app", directory: "apps" },
	{ kind: "package", directory: "packages" },
	{ kind: "service", directory: "services" },
] as const;

export const ProjectInspector: React.FC<ProjectInspectorProps> = ({
	environments,
	projects,
	environment,
	workspaceEntryId,
	readOnly,
}) => {
	const { t } = useTranslation();
	const dirtyOwner = useId();
	const [collapsedGroups, setCollapsedGroups] = useState<
		Partial<Record<OverviewProject["kind"], boolean>>
	>({});
	const [createOpen, setCreateOpen] = useState(false);
	const [bindingOpen, setBindingOpen] = useState(false);
	const binding = useSWR(workspaceEnvironmentKey(workspaceEntryId), () =>
		getWorkspaceEnvironment(workspaceEntryId),
	);
	const canCreate = !!workspaceEntryId && !readOnly;
	const [dirty, setDirty] = useState(false);
	const [pendingAction, setPendingAction] = useState<(() => void) | null>(null);
	const [searchParams, setSearchParams] = useSearchParams();
	const selectedName = searchParams.get("project") ?? projects[0]?.name ?? "";
	function setSelectedName(name: string) {
		setSearchParams(
			(current) => {
				const next = new URLSearchParams(current);
				next.set("project", name);
				next.delete("tab");
				return next;
			},
			{ replace: true },
		);
	}
	const setEnvironmentDirty = useEnvironmentDirtyStore((state) => state.setDirty);
	const clearEnvironmentDirty = useEnvironmentDirtyStore((state) => state.clearOwner);
	const selectedProject = projects.find((project) => project.name === selectedName) ?? projects[0];

	function setInspectorDirty(next: boolean) {
		setDirty(next);
		setEnvironmentDirty(dirtyOwner, next, () => setDirty(false));
	}

	useEffect(
		() => () => {
			clearEnvironmentDirty(dirtyOwner);
		},
		[clearEnvironmentDirty, dirtyOwner],
	);

	function requestDiscard(action: () => void) {
		if (!dirty) {
			action();
			return;
		}
		setPendingAction(() => action);
	}

	function selectProject(name: string) {
		if (name === selectedProject?.name) return;
		requestDiscard(() => {
			setInspectorDirty(false);
			setSelectedName(name);
		});
	}

	return (
		<>
			<section
				role="region"
				aria-label={t("projectInspector.workspaceTitle")}
				className="flex h-full min-h-0 flex-col overflow-hidden bg-background ud-md:grid ud-md:grid-cols-[208px_minmax(0,1fr)] ud-lg:grid-cols-[240px_minmax(0,1fr)]"
			>
				<aside className="hidden min-h-0 min-w-0 flex-col overflow-hidden border-r border-border bg-card ud-md:flex">
					<div className="px-4 pt-5 pb-3">
						<h2 className="text-sm font-semibold">{t("projectInspector.projectsTitle")}</h2>
					</div>
					<nav
						aria-label={t("projectInspector.projectListLabel")}
						className="grid min-h-0 min-w-0 flex-1 grid-cols-[minmax(0,1fr)] content-start gap-2 overflow-y-auto px-2 pb-3"
					>
						{PROJECT_GROUPS.map((group) => {
							const groupProjects = projects.filter((project) => project.kind === group.kind);
							if (groupProjects.length === 0) return null;
							const expanded = !collapsedGroups[group.kind];
							return (
								<Collapsible.Root
									key={group.kind}
									className="min-w-0"
									open={expanded}
									onOpenChange={(open) => {
										setCollapsedGroups((current) => ({ ...current, [group.kind]: !open }));
									}}
								>
									<Collapsible.Trigger asChild>
										<Button
											variant="ghost"
											size="sm"
											className="w-full min-w-0 justify-start"
											aria-label={t("projectInspector.groupLabel", {
												group: group.directory,
												count: groupProjects.length,
											})}
										>
											<ChevronRight
												data-icon="inline-start"
												className={cn("transition-transform", expanded && "rotate-90")}
											/>
											<span className="flex-1 text-left font-mono">{group.directory}</span>
											<span className="text-muted-foreground">{groupProjects.length}</span>
										</Button>
									</Collapsible.Trigger>
									<Collapsible.Content className="grid min-w-0 grid-cols-[minmax(0,1fr)] gap-0.5 pt-1">
										{groupProjects.map((project) => {
											const Icon = PROJECT_KIND_ICON[project.kind];
											const selected = project.name === selectedProject?.name;
											return (
												<Button
													key={project.name}
													type="button"
													variant="navigation"
													size="navigation-compact"
													aria-label={`${project.name} ${project.relativeDir}`}
													title={project.relativeDir}
													aria-current={selected ? "page" : undefined}
													onClick={() => selectProject(project.name)}
												>
													<Icon
														data-icon="inline-start"
														aria-hidden="true"
														strokeWidth={1.75}
														className={cn("text-muted-foreground", selected && "text-primary-text")}
													/>
													<span className="min-w-0 flex-1">
														<span className="flex min-w-0 items-center gap-2">
															<span className="truncate text-sm font-semibold">{project.name}</span>
														</span>
													</span>
												</Button>
											);
										})}
									</Collapsible.Content>
								</Collapsible.Root>
							);
						})}
						{projects.length === 0 ? (
							<div className="space-y-2 p-3 text-sm text-muted-foreground">
								<p>{t("projectInspector.empty")}</p>
							</div>
						) : null}
						{canCreate && (
							<Button
								className="mx-2 mt-2"
								variant="outline"
								onClick={() => requestDiscard(() => setCreateOpen(true))}
							>
								<Plus className="size-4" />
								{t("projectCreate.title")}
							</Button>
						)}
					</nav>
				</aside>
				<div className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
					<div className="min-h-0 min-w-0 flex-1 overflow-hidden">
						{selectedProject ? (
							<InspectorBody
								key={workspaceEntryId ?? "current"}
								project={selectedProject}
								environment={environment}
								workspaceEntryId={workspaceEntryId}
								readOnly={readOnly}
								onDirtyChange={setInspectorDirty}
								projectSelector={
									<div className="min-w-0 flex-1 ud-md:hidden">
										<Select
											value={selectedProject?.name ?? ""}
											onValueChange={selectProject}
											disabled={projects.length === 0}
										>
											<SelectTrigger aria-label={t("projectInspector.projectListLabel")}>
												<SelectValue placeholder={t("projectInspector.empty")} />
											</SelectTrigger>
											<SelectContent>
												{projects.map((project) => (
													<SelectItem key={project.name} value={project.name}>
														{project.name}
													</SelectItem>
												))}
											</SelectContent>
										</Select>
									</div>
								}
								actions={
									<div className="flex max-w-full flex-wrap items-center gap-2">
										{canCreate && (
											<Button
												className="ud-md:hidden"
												variant="outline"
												onClick={() => requestDiscard(() => setCreateOpen(true))}
											>
												<Plus className="size-4" />
												{t("projectCreate.title")}
											</Button>
										)}
										<EnvironmentSelector environments={environments} />
										<Button
											variant="outline"
											disabled={readOnly || binding.isLoading}
											onClick={() => setBindingOpen(true)}
										>
											<KeyRound data-icon="inline-start" />
											{t(binding.data?.projectId ? "binding.change" : "binding.workspaceTitle")}
										</Button>
									</div>
								}
							/>
						) : (
							<div className="grid h-full min-h-0 place-items-center p-5 text-center text-sm text-muted-foreground">
								<div className="space-y-3">
									<p>{t("projectInspector.empty")}</p>
									{canCreate && (
										<Button
											variant="outline"
											className="ud-md:hidden"
											onClick={() => setCreateOpen(true)}
										>
											<Plus className="size-4" />
											{t("projectCreate.title")}
										</Button>
									)}
								</div>
							</div>
						)}
					</div>
				</div>
			</section>

			<InfisicalBindingDialog
				open={bindingOpen}
				onOpenChange={setBindingOpen}
				workspaceEntryId={workspaceEntryId}
				environment={environment}
				readOnly={readOnly}
			/>

			{createOpen && canCreate && workspaceEntryId && (
				<CreateProjectDialog
					entryId={workspaceEntryId}
					projectNames={projects.map((project) => project.name)}
					onClose={() => setCreateOpen(false)}
					onCreated={(name) => {
						setInspectorDirty(false);
						setSelectedName(name);
					}}
				/>
			)}

			<AlertDialog
				open={pendingAction !== null}
				onOpenChange={(open) => {
					if (!open) setPendingAction(null);
				}}
			>
				<AlertDialogContent>
					<AlertDialogHeader>
						<AlertDialogTitle>
							{t("projectInspector.unsavedChangesTitle", {
								defaultValue: "Discard unsaved changes?",
							})}
						</AlertDialogTitle>
						<AlertDialogDescription>{t("projectInspector.unsavedChanges")}</AlertDialogDescription>
					</AlertDialogHeader>
					<AlertDialogFooter>
						<AlertDialogCancel>{t("form.cancel")}</AlertDialogCancel>
						<AlertDialogAction
							onClick={() => {
								const action = pendingAction;
								setPendingAction(null);
								setInspectorDirty(false);
								action?.();
							}}
						>
							{t("projectInspector.discardAndContinue", {
								defaultValue: "Discard and continue",
							})}
						</AlertDialogAction>
					</AlertDialogFooter>
				</AlertDialogContent>
			</AlertDialog>
		</>
	);
};

const PROJECT_KIND_ICON: Record<OverviewProject["kind"], LucideIcon> = {
	app: AppWindow,
	service: Server,
	package: Package,
};

const InspectorBody: React.FC<{
	project: OverviewProject;
	environment: string;
	workspaceEntryId?: string;
	readOnly?: boolean;
	onDirtyChange(dirty: boolean): void;
	projectSelector: React.ReactNode;
	actions: React.ReactNode;
}> = ({
	project,
	environment,
	workspaceEntryId,
	readOnly,
	onDirtyChange,
	projectSelector,
	actions,
}) => {
	const { t } = useTranslation();
	const key = projectSettingsKey(project.name, workspaceEntryId, environment);
	const result = useSWR(
		key,
		() => getProjectSettings(project.name, workspaceEntryId, environment),
		{
			keepPreviousData: true,
		},
	);
	const draft = useManifestDraftStore((state) => state.drafts[manifestDraftKey(workspaceEntryId)]);
	// Keep the layout mounted while SWR loads the next project's settings.
	// Identity must follow the selection so secrets never use the previous scope.
	const displayedProject = result.data
		? { ...result.data.project, name: project.name, relativeDir: project.relativeDir }
		: undefined;

	return (
		<div className="@container h-full min-h-0 overflow-y-auto p-3 ud-md:p-4">
			<div className="mx-auto flex h-full min-h-0 max-w-6xl flex-col gap-3">
				{result.isLoading && !result.data ? <InspectorLoading /> : null}
				{result.error ? <InspectorError onRetry={() => void result.mutate()} /> : null}
				{result.data && displayedProject && (
					<>
						<GeneralForm
							project={displayedProject}
							projectSelector={projectSelector}
							actions={actions}
						/>
						{(readOnly || draft) && (
							<div className="flex flex-wrap items-center justify-end gap-2">
								{readOnly ? (
									<Badge variant="muted">
										<LockKeyhole className="size-3" />
										{t("projectInspector.manifestReadOnly")}
									</Badge>
								) : draft ? (
									<Badge variant="warning">{t("projectInspector.manifestDraft")}</Badge>
								) : null}
								{workspaceEntryId && !readOnly ? (
									<ManifestSaveControl entryId={workspaceEntryId} />
								) : null}
							</div>
						)}
						<EnvironmentForm
							project={displayedProject}
							revision={result.data.revision}
							environment={environment}
							workspaceEntryId={workspaceEntryId}
							readOnly={readOnly || result.isLoading || !!result.error}
							onUpdated={(next) => {
								onDirtyChange(false);
								void result.mutate(next, { revalidate: false });
							}}
							onDirtyChange={onDirtyChange}
						/>
					</>
				)}
			</div>
		</div>
	);
};

const InspectorLoading: React.FC = () => {
	const { t } = useTranslation();
	return (
		<div role="status" className="space-y-4" aria-label={t("workspaces.loading")}>
			<Skeleton className="h-5 w-40" />
			<Skeleton className="h-20" />
			<Skeleton className="h-32" />
		</div>
	);
};

const InspectorError: React.FC<{ onRetry(): void }> = ({ onRetry }) => {
	const { t } = useTranslation();
	return (
		<Alert variant="destructive">
			<AlertTitle>{t("projectInspector.loadFailed")}</AlertTitle>
			<AlertDescription>
				<Button variant="outline" size="sm" className="mt-3" onClick={onRetry}>
					{t("projectInspector.retry")}
				</Button>
			</AlertDescription>
		</Alert>
	);
};
