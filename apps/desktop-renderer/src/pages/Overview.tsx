import { Combobox as ComboboxPrimitive } from "@base-ui/react/combobox";
import {
	AppWindow,
	ChevronDown,
	FolderOpen,
	Layers3,
	LockKeyhole,
	Package,
	Server,
} from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useLocation, useNavigate } from "react-router-dom";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Combobox, ComboboxContent, ComboboxItem, ComboboxList } from "@/components/ui/combobox";
import { DiscardDialog } from "@/components/ui/discard-dialog";
import { PageContent, PageHeader, StatePanel, SearchInput } from "@/components/ui/page-layout";
import {
	environmentFromSearch,
	preserveEnvironment,
} from "@/features/environment-context/environment";
import { useEnvironmentDirtyStore } from "@/features/environment-context/environment-dirty-store";
import { InfisicalBindingDialog } from "@/features/infisical-binding/InfisicalBindingDialog";
import { ManifestSaveControl } from "@/features/manifest-draft/ManifestSaveControl";
import { useManifestDraftStore } from "@/features/manifest-draft/manifest-draft-store";
import { ProjectInspector } from "@/features/project-settings/ProjectInspector";
import { workspaceVariablesPath } from "@/router/paths";
import type { OverviewProject } from "@/types/api";

const PROJECT_GROUPS = [
	{ kind: "app", directory: "apps", icon: AppWindow },
	{ kind: "package", directory: "packages", icon: Package },
	{ kind: "service", directory: "services", icon: Server },
] as const;

export function Overview({
	project,
	projects,
	workspaceEntryId,
	readOnly,
}: {
	project?: OverviewProject;
	projects: OverviewProject[];
	workspaceEntryId: string;
	readOnly?: boolean;
}) {
	const { t } = useTranslation();
	const { search } = useLocation();
	const environment = environmentFromSearch(search);
	const [bindingOpen, setBindingOpen] = useState(false);
	const [discardOpen, setDiscardOpen] = useState(false);
	const [bindingVersion, setBindingVersion] = useState(0);
	const draft = useManifestDraftStore((state) => state.drafts[workspaceEntryId]);
	const storageAction = (
		<Button
			variant="outline"
			disabled={readOnly}
			onClick={() => {
				if (useEnvironmentDirtyStore.getState().dirty) setDiscardOpen(true);
				else setBindingOpen(true);
			}}
		>
			{t("binding.storageProject")}
		</Button>
	);
	return (
		<PageContent scroll>
			<PageHeader
				title={t("navigation.environmentVariables")}
				actions={
					<>
						{readOnly ? (
							<Badge variant="muted">
								<LockKeyhole />
								{t("projectInspector.manifestReadOnly")}
							</Badge>
						) : draft ? (
							<>
								<Badge variant="warning">{t("projectInspector.manifestDraft")}</Badge>
								<ManifestSaveControl entryId={workspaceEntryId} />
							</>
						) : null}
						{storageAction}
					</>
				}
			/>
			{readOnly && (
				<Alert>
					<Layers3 />
					<AlertTitle>{t("workspaces.conflict.title")}</AlertTitle>
					<AlertDescription>{t("workspaces.conflict.description")}</AlertDescription>
				</Alert>
			)}
			{project ? (
				<ProjectInspector
					key={`${workspaceEntryId}/${project.name}/${environment}`}
					project={project}
					environment={environment}
					workspaceEntryId={workspaceEntryId}
					readOnly={readOnly}
					bindingVersion={bindingVersion}
					modulePicker={
						<WorkspaceModulePicker
							projects={projects}
							selectedProject={project}
							workspaceEntryId={workspaceEntryId}
						/>
					}
				/>
			) : (
				<StatePanel
					icon={FolderOpen}
					title={t("projectInspector.empty")}
					description={t("navigation.emptyWorkspace")}
					framed
				/>
			)}
			{bindingOpen && !readOnly && (
				<InfisicalBindingDialog
					open={bindingOpen}
					onOpenChange={setBindingOpen}
					workspaceEntryId={workspaceEntryId}
					environment={environment}
					onBound={() => setBindingVersion((version) => version + 1)}
				/>
			)}
			<DiscardDialog
				open={discardOpen && !readOnly}
				onOpenChange={setDiscardOpen}
				onDiscard={() => {
					useEnvironmentDirtyStore.getState().discardAll();
					setBindingOpen(true);
				}}
			/>
		</PageContent>
	);
}

type ModuleGroup = {
	directory: string;
	icon: typeof AppWindow;
	items: OverviewProject[];
};

function WorkspaceModulePicker({
	projects,
	selectedProject,
	workspaceEntryId,
}: {
	projects: OverviewProject[];
	selectedProject: OverviewProject;
	workspaceEntryId: string;
}) {
	const { t } = useTranslation();
	const navigate = useNavigate();
	const { search } = useLocation();
	const groups = PROJECT_GROUPS.map(({ kind, directory, icon }) => ({
		directory,
		icon,
		items: projects.filter((project) => project.kind === kind),
	})).filter((group) => group.items.length > 0);
	const path = selectedProject.relativeDir || selectedProject.name;
	return (
		<Combobox<OverviewProject>
			items={groups}
			value={selectedProject}
			itemToStringLabel={(project) => `${project.name} ${project.relativeDir}`}
			isItemEqualToValue={(project, selected) => project.name === selected.name}
			onValueChange={(project) => {
				if (!project || project.name === selectedProject.name) return;
				void navigate(
					preserveEnvironment(workspaceVariablesPath(workspaceEntryId, project.name), search),
				);
			}}
		>
			<ComboboxPrimitive.Trigger
				render={<Button variant="outline" className="h-9 max-w-full min-w-0 justify-between" />}
				aria-label={t("projectInspector.selectModule", { module: path })}
				title={path}
			>
				<span className="max-w-[min(50vw,16rem)] truncate text-left font-mono text-xs">{path}</span>
				<ChevronDown aria-hidden="true" className="text-muted-foreground" />
			</ComboboxPrimitive.Trigger>
			<ComboboxContent className="w-80">
				<div className="border-b border-border p-2">
					<ComboboxPrimitive.Input
						render={<SearchInput />}
						aria-label={t("projectInspector.searchModules")}
						placeholder={t("projectInspector.searchModules")}
					/>
				</div>
				<ComboboxPrimitive.Empty className="p-3 text-sm text-muted-foreground empty:hidden">
					{t("projectInspector.noMatchingModules")}
				</ComboboxPrimitive.Empty>
				<ComboboxList className="max-h-[min(19rem,calc(var(--available-height)-3.25rem))]">
					{({ directory, icon: Icon, items }: ModuleGroup) => (
						<ComboboxPrimitive.Group key={directory} items={items} className="px-1 pb-1">
							<ComboboxPrimitive.GroupLabel className="px-2 py-1 font-mono text-xs text-muted-foreground">
								{directory}
							</ComboboxPrimitive.GroupLabel>
							<ComboboxPrimitive.Collection>
								{(project: OverviewProject) => (
									<ComboboxItem key={project.name} value={project}>
										<Icon aria-hidden="true" className="size-4 shrink-0 text-muted-foreground" />
										<span className="min-w-0">
											<span className="block truncate">{project.name}</span>
											<span className="block truncate font-mono text-xs text-muted-foreground">
												{project.relativeDir}
											</span>
										</span>
									</ComboboxItem>
								)}
							</ComboboxPrimitive.Collection>
						</ComboboxPrimitive.Group>
					)}
				</ComboboxList>
			</ComboboxContent>
		</Combobox>
	);
}
