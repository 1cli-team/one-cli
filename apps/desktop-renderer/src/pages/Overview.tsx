import { AppWindow, ChevronRight, FolderOpen, Layers3, Package, Server } from "lucide-react";
import { Collapsible } from "radix-ui";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useLocation } from "react-router-dom";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { DiscardDialog } from "@/components/ui/discard-dialog";
import { StatePanel } from "@/components/ui/page-layout";
import { EnvironmentLink } from "@/features/environment-context/EnvironmentLink";
import { environmentFromSearch } from "@/features/environment-context/environment";
import { useEnvironmentDirtyStore } from "@/features/environment-context/environment-dirty-store";
import { InfisicalBindingDialog } from "@/features/infisical-binding/InfisicalBindingDialog";
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
		<div className="@container flex h-full min-h-0 w-full flex-col">
			{readOnly && (
				<Alert className="shrink-0 rounded-none border-x-0 border-t-0">
					<Layers3 />
					<AlertTitle>{t("workspaces.conflict.title")}</AlertTitle>
					<AlertDescription>{t("workspaces.conflict.description")}</AlertDescription>
				</Alert>
			)}
			{project ? (
				<div className="grid min-h-0 flex-1 grid-rows-[auto_minmax(0,1fr)] @min-[40rem]:grid-cols-[13rem_minmax(0,1fr)] @min-[40rem]:grid-rows-[minmax(0,1fr)]">
					<WorkspaceProjectTree
						projects={projects}
						selectedProject={project.name}
						workspaceEntryId={workspaceEntryId}
					/>
					<div className="min-h-0 min-w-0">
						<ProjectInspector
							key={`${workspaceEntryId}/${project.name}/${environment}`}
							project={project}
							environment={environment}
							workspaceEntryId={workspaceEntryId}
							readOnly={readOnly}
							storageAction={storageAction}
							bindingVersion={bindingVersion}
						/>
					</div>
				</div>
			) : (
				<div className="min-h-0 flex-1 overflow-y-auto p-4 ud-md:p-6">
					<div className="mx-auto flex max-w-[1120px] justify-end">{storageAction}</div>
					<StatePanel
						icon={FolderOpen}
						heading="h1"
						title={t("projectInspector.empty")}
						description={t("navigation.emptyWorkspace")}
					/>
				</div>
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
		</div>
	);
}

function WorkspaceProjectTree({
	projects,
	selectedProject,
	workspaceEntryId,
}: {
	projects: OverviewProject[];
	selectedProject: string;
	workspaceEntryId: string;
}) {
	const { t } = useTranslation();
	return (
		<nav
			aria-label={t("projectInspector.projectListLabel")}
			className="flex max-h-48 min-w-0 flex-col gap-1 overflow-y-auto border-b border-border bg-card p-3 @min-[40rem]:max-h-none @min-[40rem]:border-r @min-[40rem]:border-b-0"
		>
			{PROJECT_GROUPS.map(({ kind, directory, icon: Icon }) => {
				const group = projects.filter((project) => project.kind === kind);
				if (!group.length) return null;
				return (
					<Collapsible.Root key={kind} defaultOpen>
						<Collapsible.Trigger asChild>
							<Button variant="ghost" size="sm" className="group w-full justify-start">
								<ChevronRight
									aria-hidden="true"
									className="transition-transform group-data-[state=open]:rotate-90 motion-reduce:transition-none"
								/>
								<span className="flex-1 text-left font-mono">{directory}</span>
								<span className="tabular-nums text-muted-foreground">{group.length}</span>
							</Button>
						</Collapsible.Trigger>
						<Collapsible.Content className="ml-3 flex flex-col gap-1 border-l border-border pl-2">
							{group.map((project) => (
								<Button key={project.name} variant="navigation" size="navigation-compact" asChild>
									<EnvironmentLink
										to={workspaceVariablesPath(workspaceEntryId, project.name)}
										aria-current={selectedProject === project.name ? "page" : undefined}
										title={project.relativeDir}
									>
										<Icon aria-hidden="true" />
										<span className="min-w-0 truncate">{project.name}</span>
									</EnvironmentLink>
								</Button>
							))}
						</Collapsible.Content>
					</Collapsible.Root>
				);
			})}
		</nav>
	);
}
