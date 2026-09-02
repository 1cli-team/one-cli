import {
	Boxes,
	CloudUpload,
	Code2,
	KeyRound,
	Library,
	MoonStar,
	Settings2,
	SunMedium,
} from "lucide-react";
import type React from "react";
import { useEffect, useId, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import useSWR from "swr";
import { backendRequiresContainerArtifact, useBackendCatalog } from "@/api/catalog";
import { getProjectSettings, projectSettingsKey } from "@/api/workspace";
import { LanguageSwitcher } from "@/components/LanguageSwitcher";
import { ManifestSaveControl } from "@/components/TopBar";
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
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { EnvironmentLink } from "@/features/environment-context/EnvironmentLink";
import { EnvironmentSelector } from "@/features/environment-context/EnvironmentSelector";
import { useEnvironmentDirtyStore } from "@/features/environment-context/environment-dirty-store";
import {
	manifestDraftKey,
	useManifestDraftStore,
} from "@/features/manifest-draft/manifest-draft-store";
import { ContainerForm } from "@/features/project-settings/forms/ContainerForm";
import { DeployForm } from "@/features/project-settings/forms/DeployForm";
import { EnvironmentForm } from "@/features/project-settings/forms/EnvironmentForm";
import { GeneralForm } from "@/features/project-settings/forms/GeneralForm";
import type { ProjectInspectorTab } from "@/features/project-settings/ProjectMatrix";
import { WorkspaceSettingsDialog } from "@/features/workspace-settings/WorkspaceSettingsDialog";
import { useThemeStore } from "@/lib/stores/theme";
import { cn } from "@/lib/utils";
import type { OverviewProject, ProjectSettingsResponse } from "@/types/api";

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
	{ id: "overview", icon: Settings2 },
	{ id: "environment", icon: KeyRound },
	{ id: "deploy", icon: CloudUpload },
];

export const ProjectInspector: React.FC<ProjectInspectorProps> = ({
	projects,
	currentBackend,
	environment,
	workspaceEntryId,
	readOnly,
}) => {
	const { t } = useTranslation();
	const { mode, toggle } = useThemeStore();
	const dirtyOwner = useId();
	const [dirty, setDirty] = useState(false);
	const [pendingAction, setPendingAction] = useState<(() => void) | null>(null);
	const [selectedName, setSelectedName] = useState(projects[0]?.name ?? "");
	const setEnvironmentDirty = useEnvironmentDirtyStore((state) => state.setDirty);
	const clearEnvironmentDirty = useEnvironmentDirtyStore((state) => state.clearOwner);
	const selectedProject = projects.find((project) => project.name === selectedName) ?? projects[0];
	const logoSrc = mode === "dark" ? "/brand/icon-inverted.svg" : "/brand/icon.svg";

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

	return (
		<>
			<section
				role="region"
				aria-label={t("projectInspector.workspaceTitle")}
				className="grid h-full min-h-0 grid-cols-[244px_minmax(0,1fr)] overflow-hidden bg-card"
			>
				<aside className="flex min-h-0 flex-col border-r border-border bg-muted/30">
					<EnvironmentLink
						to="/"
						className="flex min-h-16 items-center gap-2.5 px-4 outline-none transition-colors hover:bg-muted/60 focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-inset"
						aria-label={t("topbar.workspaces")}
					>
						<img src={logoSrc} alt="One CLI" className="size-8 shrink-0" />
						<div>
							<h1 className="font-heading text-base font-semibold leading-none tracking-tight">
								One CLI
							</h1>
							<p className="mt-1 font-mono text-[9px] font-medium tracking-[0.12em] text-muted-foreground uppercase">
								{t("sidebar.brand")}
							</p>
						</div>
					</EnvironmentLink>
					<nav
						aria-label={t("projectInspector.projectListLabel")}
						className="grid min-h-0 flex-1 content-start gap-1 overflow-y-auto px-2 pt-5 pb-2"
					>
						{projects.map((project) => {
							const Icon = PROJECT_KIND_ICON[project.kind];
							const selected = project.name === selectedProject?.name;
							return (
								<Button
									key={project.name}
									type="button"
									variant="ghost"
									aria-label={`${project.name} ${project.relativeDir}`}
									aria-current={selected ? "page" : undefined}
									onClick={() =>
										requestDiscard(() => {
											setInspectorDirty(false);
											setSelectedName(project.name);
										})
									}
									className={cn(
										"relative h-auto min-h-14 w-full justify-start gap-2.5 rounded-lg border border-transparent px-3 py-2 text-left font-normal hover:border-border hover:bg-card/80",
										selected &&
											"border-primary/20 bg-card shadow-sm before:absolute before:inset-y-2 before:left-0 before:w-[3px] before:rounded-r before:bg-primary",
									)}
								>
									<span
										className={cn(
											"grid size-7 shrink-0 place-items-center rounded-md border border-border/80 bg-muted/50 text-muted-foreground",
											selected && "border-primary/15 bg-primary/8 text-primary",
										)}
									>
										<Icon className="size-3.5" />
									</span>
									<span className="min-w-0 flex-1">
										<span className="block truncate text-sm font-semibold">{project.name}</span>
										<span className="mt-0.5 block truncate font-mono text-[10px] text-muted-foreground">
											{project.relativeDir}
										</span>
									</span>
								</Button>
							);
						})}
					</nav>
					<div className="mt-auto grid h-12 shrink-0 grid-cols-4 place-items-center border-t border-border px-3">
						<EnvironmentSelector variant="icon" />
						<WorkspaceSettingsDialog
							currentBackend={currentBackend}
							environment={environment}
							projects={projects}
							workspaceEntryId={workspaceEntryId}
							readOnly={readOnly}
							triggerVariant="icon"
						/>
						<LanguageSwitcher />
						<Button
							type="button"
							variant="ghost"
							size="icon-sm"
							onClick={toggle}
							title={mode === "light" ? t("sidebar.themeToDark") : t("sidebar.themeToLight")}
							aria-label={mode === "light" ? t("sidebar.themeToDark") : t("sidebar.themeToLight")}
						>
							{mode === "light" ? <MoonStar /> : <SunMedium />}
						</Button>
					</div>
				</aside>

				<div className="min-h-0 min-w-0 overflow-hidden">
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
			</section>

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
	service: Boxes,
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
	const sectionTitle = t(
		`projectInspector.${activeTab === "overview" ? "general" : activeTab}.title`,
	);
	const isManifestDraftSection = activeTab !== "deploy";

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
			<div className="flex min-h-16 shrink-0 flex-col justify-between gap-3 border-b border-border px-4 py-3 sm:flex-row sm:items-center">
				<div className="min-w-0">
					<div className="flex items-center gap-2">
						<h2 className="font-heading text-lg font-semibold tracking-tight">{sectionTitle}</h2>
						{isManifestDraftSection ? (
							<Badge variant="secondary" className="font-mono text-[10px]">
								{t("projectInspector.manifestDraft")}
							</Badge>
						) : null}
					</div>
				</div>
				<div className="flex items-center gap-2">
					<TabsList
						variant="default"
						className="h-9 max-w-full justify-start gap-1 overflow-x-auto rounded-lg border border-border/70 bg-muted/60 p-0.5"
						aria-label={t("projectInspector.tabs.label")}
					>
						{TAB_ITEMS.map(({ id, icon: Icon }) => (
							<TabsTrigger
								key={id}
								value={id}
								className="h-8 flex-none rounded-md border-0 bg-transparent px-2.5 text-xs text-muted-foreground shadow-none hover:text-foreground data-[state=active]:bg-card data-[state=active]:text-foreground data-[state=active]:shadow-sm"
							>
								<Icon className="h-3.5 w-3.5" />
								{t(`projectInspector.tabs.${id}`)}
							</TabsTrigger>
						))}
					</TabsList>
					{workspaceEntryId ? <ManifestSaveControl entryId={workspaceEntryId} /> : null}
				</div>
			</div>

			<div className="min-h-0 flex-1 overflow-y-auto bg-muted/[0.1] p-4">
				{TAB_ITEMS.map(({ id }) => (
					<TabsContent key={id} value={id} className="mt-0 outline-none">
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

const InspectorLoading: React.FC = () => (
	<div className="space-y-4" aria-label="Loading">
		<Skeleton className="h-5 w-40" />
		<Skeleton className="h-20" />
		<Skeleton className="h-32" />
	</div>
);

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
				key={project.environment.selectedProfile ?? ""}
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
	return (
		<DeploymentSettingsPanel
			project={project}
			revision={data.revision}
			environment={environment}
			workspaceEntryId={workspaceEntryId}
			readOnly={readOnly}
			onUpdated={onUpdated}
			onDirtyChange={onDirtyChange}
		/>
	);
};

type ProfileSection = "deploy" | "container";

const DeploymentSettingsPanel: React.FC<
	Omit<ProjectSettingsPanelProps, "data" | "activeTab"> & {
		project: ProjectSettingsResponse["project"];
		revision: string;
	}
> = ({ project, revision, environment, workspaceEntryId, readOnly, onUpdated, onDirtyChange }) => {
	const catalog = useBackendCatalog();
	const dirtySections = useRef<Record<ProfileSection, boolean>>({
		deploy: false,
		container: false,
	});
	const [containerProfileDirty, setContainerProfileDirty] = useState(false);
	const stagedDeploy = useManifestDraftStore(
		(state) => state.drafts[manifestDraftKey(workspaceEntryId)]?.changes[project.name]?.deploy,
	);
	const deployBackend = stagedDeploy?.backend ?? project.deploy.backend;
	const requiresImage = backendRequiresContainerArtifact(
		deployBackend ? catalog.byID.get(`deploy/${deployBackend}`) : undefined,
	);

	function setSectionDirty(section: ProfileSection, dirty: boolean) {
		dirtySections.current[section] = dirty;
		if (section === "container") setContainerProfileDirty(dirty);
		onDirtyChange(dirtySections.current.deploy || dirtySections.current.container);
	}

	function sectionUpdated(section: ProfileSection, next: ProjectSettingsResponse) {
		setSectionDirty(section, false);
		onUpdated(next);
	}

	return (
		<DeployForm
			project={project}
			revision={revision}
			environment={environment}
			workspaceEntryId={workspaceEntryId}
			readOnly={readOnly}
			onUpdated={(next) => sectionUpdated("deploy", next)}
			onDirtyChange={(dirty) => setSectionDirty("deploy", dirty)}
		>
			{requiresImage || containerProfileDirty ? (
				<ContainerForm
					project={project}
					revision={revision}
					environment={environment}
					workspaceEntryId={workspaceEntryId}
					readOnly={readOnly}
					onUpdated={(next) => sectionUpdated("container", next)}
					onDirtyChange={(dirty) => setSectionDirty("container", dirty)}
				/>
			) : null}
		</DeployForm>
	);
};
