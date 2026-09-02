import { Box, CloudUpload, KeyRound, Settings2 } from "lucide-react";
import type React from "react";
import { useEffect, useId, useState } from "react";
import { useTranslation } from "react-i18next";
import useSWR from "swr";
import { getProjectSettings, projectSettingsKey } from "@/api/workspace";
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
	Sheet,
	SheetContent,
	SheetDescription,
	SheetHeader,
	SheetTitle,
} from "@/components/ui/sheet";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useEnvironmentDirtyStore } from "@/features/environment-context/environment-dirty-store";
import { ContainerForm } from "@/features/project-settings/forms/ContainerForm";
import { DeployForm } from "@/features/project-settings/forms/DeployForm";
import { EnvironmentForm } from "@/features/project-settings/forms/EnvironmentForm";
import { GeneralForm } from "@/features/project-settings/forms/GeneralForm";
import type { ProjectInspectorTab } from "@/features/project-settings/ProjectMatrix";
import type { OverviewProject, ProjectSettingsResponse } from "@/types/api";

export interface ProjectInspectorTarget {
	project: OverviewProject;
	tab: ProjectInspectorTab;
}

interface ProjectInspectorProps {
	target: ProjectInspectorTarget | null;
	environment: string;
	workspaceEntryId?: string;
	readOnly?: boolean;
	onOpenChange(open: boolean): void;
}

const TAB_ITEMS: ReadonlyArray<{
	id: ProjectInspectorTab;
	icon: React.ComponentType<{ className?: string }>;
}> = [
	{ id: "overview", icon: Settings2 },
	{ id: "environment", icon: KeyRound },
	{ id: "container", icon: Box },
	{ id: "deploy", icon: CloudUpload },
];

export const ProjectInspector: React.FC<ProjectInspectorProps> = ({
	target,
	environment,
	workspaceEntryId,
	readOnly,
	onOpenChange,
}) => {
	const { t } = useTranslation();
	const dirtyOwner = useId();
	const [dirty, setDirty] = useState(false);
	const [pendingAction, setPendingAction] = useState<(() => void) | null>(null);
	const setEnvironmentDirty = useEnvironmentDirtyStore((state) => state.setDirty);
	const clearEnvironmentDirty = useEnvironmentDirtyStore((state) => state.clearOwner);

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

	function closeInspector() {
		setInspectorDirty(false);
		onOpenChange(false);
	}

	return (
		<>
			<Sheet
				modal={false}
				open={target !== null}
				onOpenChange={(open) => {
					if (open) {
						onOpenChange(true);
						return;
					}
					requestDiscard(closeInspector);
				}}
			>
				<SheetContent
					className="top-[68px] h-[calc(100%-68px)] w-[620px]"
					overlayClassName="top-[68px]"
					closeLabel={t("projectInspector.close")}
					onInteractOutside={(event) => event.preventDefault()}
				>
					{target ? (
						<InspectorBody
							key={`${workspaceEntryId ?? "current"}:${environment}:${target.project.name}:${target.tab}`}
							project={target.project}
							environment={environment}
							workspaceEntryId={workspaceEntryId}
							readOnly={readOnly}
							initialTab={target.tab}
							onDirtyChange={setInspectorDirty}
							onRequestDiscard={requestDiscard}
						/>
					) : null}
				</SheetContent>
			</Sheet>

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
			className="min-h-0 flex-1 gap-0"
		>
			<div className="shrink-0 border-b border-border px-6 pb-0 pt-5">
				<SheetHeader className="p-0 pr-10">
					<div className="flex items-center gap-2">
						<SheetTitle>{project.name}</SheetTitle>
						<Badge variant="outline">{t(`overview.kinds.${project.kind}`)}</Badge>
					</div>
					<SheetDescription className="font-mono text-xs">
						{project.relativeDir} · {environment}
					</SheetDescription>
				</SheetHeader>
				<TabsList
					variant="line"
					className="mt-5 h-10 justify-start gap-5 rounded-none bg-transparent p-0"
					aria-label={t("projectInspector.tabs.label")}
				>
					{TAB_ITEMS.map(({ id, icon: Icon }) => (
						<TabsTrigger
							key={id}
							value={id}
							className="relative h-10 flex-none rounded-none border-0 bg-transparent px-0 text-sm text-muted-foreground shadow-none after:absolute after:inset-x-0 after:bottom-0 after:h-0.5 after:bg-transparent hover:text-foreground data-[state=active]:bg-transparent data-[state=active]:text-primary data-[state=active]:shadow-none data-[state=active]:after:bg-primary"
						>
							<Icon className="h-3.5 w-3.5" />
							{t(`projectInspector.tabs.${id}`)}
						</TabsTrigger>
					))}
				</TabsList>
			</div>

			<div className="min-h-0 flex-1 overflow-y-auto px-6 py-5">
				{TAB_ITEMS.map(({ id }) => (
					<TabsContent key={id} value={id} className="mt-0 outline-none">
						{result.isLoading ? <InspectorLoading /> : null}
						{result.error ? <InspectorError onRetry={() => void result.mutate()} /> : null}
						{result.data ? (
							<ProjectSettingsPanel
								key={`${workspaceEntryId ?? "current"}:${environment}:${project.name}:${id}:${result.data.project.environment.selectedProfile ?? ""}:${result.data.project.deploy.selectedProfile ?? ""}:${result.data.project.container.selectedProfile ?? ""}`}
								data={result.data}
								environment={environment}
								workspaceEntryId={workspaceEntryId}
								readOnly={readOnly}
								activeTab={id}
								onUpdated={(next) => {
									onDirtyChange(false);
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
		<Skeleton className="h-20 rounded-lg" />
		<Skeleton className="h-32 rounded-lg" />
	</div>
);

const InspectorError: React.FC<{ onRetry(): void }> = ({ onRetry }) => {
	const { t } = useTranslation();
	return (
		<Alert variant="destructive" className="rounded-lg">
			<AlertTitle>{t("projectInspector.loadFailed")}</AlertTitle>
			<AlertDescription>
				<Button variant="outline" size="sm" className="mt-3" onClick={onRetry}>
					{t("projectInspector.retry")}
				</Button>
			</AlertDescription>
		</Alert>
	);
};

const ProjectSettingsPanel: React.FC<{
	data: ProjectSettingsResponse;
	environment: string;
	workspaceEntryId?: string;
	readOnly?: boolean;
	activeTab: ProjectInspectorTab;
	onUpdated(next: ProjectSettingsResponse): void;
	onDirtyChange(dirty: boolean): void;
}> = ({ data, environment, workspaceEntryId, readOnly, activeTab, onUpdated, onDirtyChange }) => {
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
	if (activeTab === "container") {
		return (
			<ContainerForm
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
	return (
		<DeployForm
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
