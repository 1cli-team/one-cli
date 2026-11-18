import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import useSWR from "swr";
import { getProjectSettings, projectSettingsKey } from "@/api/workspace";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { PageToolbar } from "@/components/ui/page-layout";
import { Skeleton } from "@/components/ui/skeleton";
import { EnvironmentForm } from "@/features/project-settings/forms/EnvironmentForm";
import type { OverviewProject } from "@/types/api";

export function ProjectInspector({
	project,
	environment,
	workspaceEntryId,
	readOnly,
	bindingVersion,
	modulePicker,
}: {
	project: OverviewProject;
	environment: string;
	workspaceEntryId: string;
	readOnly?: boolean;
	bindingVersion: number;
	modulePicker: ReactNode;
}) {
	const { t } = useTranslation();
	const result = useSWR(projectSettingsKey(project.name, workspaceEntryId, environment), () =>
		getProjectSettings(project.name, workspaceEntryId, environment),
	);
	const settings = result.data?.project;
	return (
		<>
			{settings?.environment.backend !== "infisical" && <PageToolbar>{modulePicker}</PageToolbar>}
			{result.isLoading && !result.data && (
				<div role="status" aria-label={t("workspaces.loading")} className="flex flex-col gap-4">
					<Skeleton className="h-20" />
					<Skeleton className="h-64" />
				</div>
			)}
			{result.error && (
				<Alert variant="destructive">
					<AlertTitle>{t("projectInspector.loadFailed")}</AlertTitle>
					<AlertDescription>
						<div className="mt-3 flex flex-wrap gap-2">
							<Button variant="outline" onClick={() => void result.mutate()}>
								{t("projectInspector.retry")}
							</Button>
						</div>
					</AlertDescription>
				</Alert>
			)}
			{settings && (
				<EnvironmentForm
					key={bindingVersion}
					project={settings}
					environment={environment}
					workspaceEntryId={workspaceEntryId}
					readOnly={readOnly || result.isLoading || !!result.error}
					toolbarStart={modulePicker}
				/>
			)}
		</>
	);
}
