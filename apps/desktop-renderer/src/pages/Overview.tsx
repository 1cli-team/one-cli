import { FolderOpen, Layers3 } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useLocation } from "react-router-dom";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { StatePanel } from "@/components/ui/page-layout";
import { environmentFromSearch } from "@/features/environment-context/environment";
import { ProjectInspector } from "@/features/project-settings/ProjectInspector";
import type { OverviewProject } from "@/types/api";

export function Overview({
	project,
	workspaceEntryId,
	readOnly,
}: {
	project?: OverviewProject;
	workspaceEntryId: string;
	readOnly?: boolean;
}) {
	const { t } = useTranslation();
	const { search } = useLocation();
	const environment = environmentFromSearch(search);
	return (
		<div className="flex h-full min-h-0 w-full flex-col">
			{readOnly && (
				<Alert className="shrink-0 rounded-none border-x-0 border-t-0">
					<Layers3 />
					<AlertTitle>{t("workspaces.conflict.title")}</AlertTitle>
					<AlertDescription>{t("workspaces.conflict.description")}</AlertDescription>
				</Alert>
			)}
			<div className="min-h-0 flex-1">
				{project ? (
					<ProjectInspector
						key={`${workspaceEntryId}/${project.name}/${environment}`}
						project={project}
						environment={environment}
						workspaceEntryId={workspaceEntryId}
						readOnly={readOnly}
					/>
				) : (
					<StatePanel
						icon={FolderOpen}
						heading="h1"
						title={t("projectInspector.empty")}
						description={t("navigation.emptyWorkspace")}
					/>
				)}
			</div>
		</div>
	);
}
