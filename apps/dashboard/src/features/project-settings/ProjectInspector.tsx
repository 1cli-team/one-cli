import {
	manifestDraftKey,
	useManifestDraftStore,
} from "@/features/manifest-draft/manifest-draft-store";
import {
	Plus,
	Terminal,
	Server,
	Code2,
	FileKey2,
	Library,
	Search,
	LayoutGrid,
	LockKeyhole,
} from "lucide-react";
import type React from "react";
import { useEffect, useId, useState } from "react";
import { useTranslation } from "react-i18next";
import useSWR from "swr";
import { getServices, servicesKey } from "@/api/services";
import { getProjectSettings, projectSettingsKey } from "@/api/workspace";
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
import { InputGroup, InputGroupAddon, InputGroupInput } from "@/components/ui/input-group";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { EnvironmentSelector } from "@/features/environment-context/EnvironmentSelector";
import { useEnvironmentDirtyStore } from "@/features/environment-context/environment-dirty-store";
import { EnvironmentForm } from "@/features/project-settings/forms/EnvironmentForm";
import { ServicePanel } from "@/features/services/ServicePanel";
import { GeneralForm } from "@/features/project-settings/forms/GeneralForm";
import { WorkspaceSettingsDialog } from "@/features/workspace-settings/WorkspaceSettingsDialog";
import { cn } from "@/lib/utils";
import type { OverviewProject, ProjectSettingsResponse } from "@/types/api";

type ProjectInspectorTab = "overview" | "environment" | "runtime";

interface ProjectInspectorProps {
	projects: OverviewProject[];
	currentBackend?: string;
	environment: string;
	workspaceEntryId?: string;
	readOnly?: boolean;
}

const TAB_ITEMS: ReadonlyArray<{
	id: ProjectInspectorTab;
	icon: React.ComponentType<{ className?: string }>;
}> = [
	{ id: "overview", icon: LayoutGrid },
	{ id: "runtime", icon: Terminal },
	{ id: "environment", icon: FileKey2 },
];

export const ProjectInspector: React.FC<ProjectInspectorProps> = ({
	projects,
	currentBackend,
	environment,
	workspaceEntryId,
	readOnly,
}) => {
	const { t } = useTranslation();
	const dirtyOwner = useId();
	const services = useSWR(servicesKey(workspaceEntryId), () => getServices(workspaceEntryId), {
		refreshInterval: 2000,
	});
	const [query, setQuery] = useState("");
	const [createOpen, setCreateOpen] = useState(false);
	const canCreate = !!workspaceEntryId && !readOnly;
	const [dirty, setDirty] = useState(false);
	const [pendingAction, setPendingAction] = useState<(() => void) | null>(null);
	const [selectedName, setSelectedName] = useState(projects[0]?.name ?? "");
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

	const filteredProjects = projects.filter((project) =>
		`${project.name} ${project.relativeDir} ${project.domains?.env ?? currentBackend ?? ""}`
			.toLocaleLowerCase()
			.includes(query.trim().toLocaleLowerCase()),
	);
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
				<aside className="hidden min-h-0 flex-col border-r border-border bg-card ud-md:flex">
					<div className="space-y-3 px-4 pt-5 pb-3">
						<div className="flex items-center justify-between">
							<h2 className="text-sm font-semibold">{t("projectInspector.projectsTitle")}</h2>
							<Badge variant="muted">{projects.length}</Badge>
						</div>
						<InputGroup>
							<InputGroupAddon>
								<Search />
							</InputGroupAddon>
							<InputGroupInput
								value={query}
								onChange={(event) => setQuery(event.target.value)}
								placeholder={t("projects.search")}
								aria-label={t("projects.search")}
							/>
						</InputGroup>
					</div>
					<nav
						aria-label={t("projectInspector.projectListLabel")}
						className="grid min-h-0 flex-1 content-start gap-1 overflow-y-auto px-2 pb-3"
					>
						{filteredProjects.map((project) => {
							const Icon = PROJECT_KIND_ICON[project.kind];
							const selected = project.name === selectedProject?.name;
							const status = services.data?.services.find(
								(service) => service.project === project.name,
							)?.status;
							return (
								<Button
									key={project.name}
									type="button"
									variant="navigation"
									size="navigation"
									aria-label={`${project.name} ${project.relativeDir}`}
									aria-current={selected ? "page" : undefined}
									onClick={() => selectProject(project.name)}
								>
									<span
										className={cn(
											"grid size-7 shrink-0 place-items-center rounded-md border border-border/80 bg-muted/50 text-muted-foreground",
											selected && "border-transparent bg-accent text-primary-text",
										)}
									>
										<Icon className="size-4" />
									</span>
									<span className="min-w-0 flex-1">
										<span className="flex min-w-0 items-center gap-2">
											<span className="truncate text-sm font-semibold">{project.name}</span>
											{status && (
												<span
													className={cn(
														"size-2 shrink-0 rounded-full",
														status === "running"
															? "bg-success-foreground"
															: status === "failed"
																? "bg-error-foreground"
																: status === "preparing" || status === "stopping"
																	? "bg-warning-foreground"
																	: "bg-muted-foreground",
													)}
													role="img"
													aria-label={t(`services.status.${status}`)}
													title={t(`services.status.${status}`)}
												/>
											)}
										</span>
										<span className="mt-0.5 block truncate font-mono text-xs text-muted-foreground">
											{project.relativeDir}
										</span>
									</span>
								</Button>
							);
						})}
						{filteredProjects.length === 0 ? (
							<div className="space-y-2 p-3 text-sm text-muted-foreground">
								<p>{query ? t("projects.empty.title") : t("projectInspector.empty")}</p>
								{query ? (
									<Button variant="outline" onClick={() => setQuery("")}>
										{t("workspaces.home.clearSearch")}
									</Button>
								) : null}
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
					<div className="flex shrink-0 flex-wrap items-center justify-between gap-3 border-b border-border bg-card px-4 py-3 ud-md:px-6">
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
						<p
							className="hidden min-w-0 flex-1 truncate text-sm font-medium ud-md:block"
							title={selectedProject?.relativeDir}
						>
							{selectedProject?.name}
							<span className="ml-3 font-mono text-xs font-normal text-muted-foreground">
								{selectedProject?.relativeDir}
							</span>
						</p>
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
							<EnvironmentSelector />
							<WorkspaceSettingsDialog
								currentBackend={currentBackend}
								environment={environment}
								projects={projects}
								workspaceEntryId={workspaceEntryId}
								readOnly={readOnly}
								triggerVariant="default"
							/>
						</div>
					</div>
					<div className="min-h-0 min-w-0 flex-1 overflow-hidden">
						{selectedProject ? (
							<InspectorBody
								key={`${workspaceEntryId ?? "current"}:${environment}:${selectedProject.name}`}
								project={selectedProject}
								environment={environment}
								workspaceEntryId={workspaceEntryId}
								readOnly={readOnly}
								initialTab="overview"
								onDirtyChange={setInspectorDirty}
								onRequestDiscard={requestDiscard}
							/>
						) : (
							<div className="grid h-full min-h-0 place-items-center p-5 text-center text-sm text-muted-foreground">
								{t("projectInspector.empty")}
							</div>
						)}
					</div>
				</div>
			</section>

			{createOpen && canCreate && workspaceEntryId && (
				<CreateProjectDialog
					entryId={workspaceEntryId}
					projectNames={projects.map((project) => project.name)}
					onClose={() => setCreateOpen(false)}
					onCreated={(name) => {
						setQuery("");
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

const PROJECT_KIND_ICON: Record<
	OverviewProject["kind"],
	React.ComponentType<{ className?: string }>
> = {
	app: Code2,
	service: Server,
	package: Library,
};

const InspectorBody: React.FC<{
	project: OverviewProject;
	environment: string;
	workspaceEntryId?: string;
	readOnly?: boolean;
	initialTab: ProjectInspectorTab;
	onDirtyChange(dirty: boolean): void;
	onRequestDiscard(action: () => void): void;
}> = ({
	project,
	environment,
	workspaceEntryId,
	readOnly,
	initialTab,
	onDirtyChange,
	onRequestDiscard,
}) => {
	const { t } = useTranslation();
	const [activeTab, setActiveTab] = useState(initialTab);
	const key = projectSettingsKey(project.name, workspaceEntryId, environment);
	const result = useSWR(key, () => getProjectSettings(project.name, workspaceEntryId, environment));
	const draft = useManifestDraftStore((state) => state.drafts[manifestDraftKey(workspaceEntryId)]);
	const sectionTitle = t(
		`projectInspector.${activeTab === "overview" ? "general" : activeTab}.title`,
	);

	return (
		<Tabs
			value={activeTab}
			onValueChange={(value) => {
				const nextTab = value as ProjectInspectorTab;
				if (nextTab === activeTab) return;
				onRequestDiscard(() => {
					onDirtyChange(false);
					setActiveTab(nextTab);
				});
			}}
			className="flex h-full min-h-0 flex-col gap-0"
		>
			<div className="shrink-0 border-b border-border bg-card px-4 pt-5 ud-md:px-6">
				<div className="mb-4 flex flex-wrap items-center justify-between gap-3">
					<h1 className="text-2xl font-semibold tracking-tight">{project.name}</h1>
					<div className="flex items-center gap-2">
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
				</div>
				<h2 className="sr-only">{sectionTitle}</h2>
				<TabsList
					variant="line"
					className="max-w-full justify-start"
					aria-label={t("projectInspector.tabs.label")}
				>
					{TAB_ITEMS.map(({ id, icon: Icon }) => (
						<TabsTrigger key={id} value={id} className="flex-none px-3">
							<Icon className="size-4" />
							{t(`projectInspector.tabs.${id}`)}
						</TabsTrigger>
					))}
				</TabsList>
			</div>

			<div className="@container min-h-0 flex-1 overflow-y-auto p-4 ud-md:p-6">
				{TAB_ITEMS.map(({ id }) => (
					<TabsContent key={id} value={id} className="mx-auto mt-0 max-w-6xl outline-none">
						{result.isLoading ? <InspectorLoading /> : null}
						{result.error ? <InspectorError onRetry={() => void result.mutate()} /> : null}
						{result.data ? (
							<ProjectSettingsPanel
								key={`${workspaceEntryId ?? "current"}:${environment}:${project.name}:${id}`}
								data={result.data}
								environment={environment}
								workspaceEntryId={workspaceEntryId}
								readOnly={readOnly}
								activeTab={id}
								onUpdated={(next) => {
									void result.mutate(next, { revalidate: false });
								}}
								onDirtyChange={onDirtyChange}
							/>
						) : null}
					</TabsContent>
				))}
			</div>
		</Tabs>
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

interface ProjectSettingsPanelProps {
	data: ProjectSettingsResponse;
	environment: string;
	workspaceEntryId?: string;
	readOnly?: boolean;
	activeTab: ProjectInspectorTab;
	onUpdated(next: ProjectSettingsResponse): void;
	onDirtyChange(dirty: boolean): void;
}

const ProjectSettingsPanel: React.FC<ProjectSettingsPanelProps> = ({
	data,
	environment,
	workspaceEntryId,
	readOnly,
	activeTab,
	onUpdated,
	onDirtyChange,
}) => {
	const project = data.project;
	if (activeTab === "runtime") {
		return (
			<ServicePanel
				project={project}
				environment={environment}
				entryId={workspaceEntryId}
				readOnly={readOnly}
			/>
		);
	}
	if (activeTab === "overview") {
		return (
			<GeneralForm
				project={project}
				revision={data.revision}
				environment={environment}
				workspaceEntryId={workspaceEntryId}
				readOnly={readOnly}
				onUpdated={onUpdated}
				onDirtyChange={onDirtyChange}
			/>
		);
	}
	if (activeTab === "environment") {
		return (
			<EnvironmentForm
				key={project.name}
				project={project}
				revision={data.revision}
				environment={environment}
				workspaceEntryId={workspaceEntryId}
				readOnly={readOnly}
				onUpdated={(next) => {
					onDirtyChange(false);
					onUpdated(next);
				}}
				onDirtyChange={onDirtyChange}
			/>
		);
	}
	return null;
};
