import { Layers3 } from "lucide-react";
import type React from "react";
import { useTranslation } from "react-i18next";
import { useLocation } from "react-router-dom";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { environmentFromSearch } from "@/features/environment-context/environment";
import { ProjectInspector } from "@/features/project-settings/ProjectInspector";
import type { Overview as OverviewPayload } from "@/types/api";

export const Overview: React.FC<{
	data: OverviewPayload;
	workspaceEntryId?: string;
	readOnly?: boolean;
}> = ({ data, workspaceEntryId, readOnly }) => {
	const { t } = useTranslation();
	const { search } = useLocation();
	const environment = environmentFromSearch(search);
	const projects = data.projects ?? [];

	return (
		<div className="flex h-full min-h-0 w-full flex-col [&_[data-slot=button]]:rounded-md [&_[data-slot=input]]:h-9 [&_[data-slot=input]]:rounded-md [&_[data-slot=select-trigger]]:h-9 [&_[data-slot=select-trigger]]:rounded-md [&_[data-slot=textarea]]:rounded-md">
			{readOnly ? (
				<Alert className="shrink-0 rounded-none border-x-0 border-t-0">
					<Layers3 className="h-4 w-4" />
					<AlertTitle>{t("workspaces.conflict.title")}</AlertTitle>
					<AlertDescription>{t("workspaces.conflict.description")}</AlertDescription>
				</Alert>
			) : null}

			<div className="min-h-0 flex-1">
				<ProjectInspector
					key={`${workspaceEntryId ?? "current"}:${environment}`}
					projects={projects}
					currentBackend={data.workspace?.domains?.env}
					environment={environment}
					workspaceEntryId={workspaceEntryId}
					readOnly={readOnly}
				/>
			</div>
		</div>
	);
};
