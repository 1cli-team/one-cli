import { LockKeyhole } from "lucide-react";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import useSWR from "swr";
import { getProjectSettings, projectSettingsKey } from "@/api/workspace";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { ManifestSaveControl } from "@/features/manifest-draft/ManifestSaveControl";
import { useManifestDraftStore } from "@/features/manifest-draft/manifest-draft-store";
import { EnvironmentForm } from "@/features/project-settings/forms/EnvironmentForm";
import { GeneralForm } from "@/features/project-settings/forms/GeneralForm";
import type { OverviewProject } from "@/types/api";

export function ProjectInspector({
	project,
	environment,
	workspaceEntryId,
	readOnly,
	storageAction,
	bindingVersion,
}: {
	project: OverviewProject;
	environment: string;
	workspaceEntryId: string;
	readOnly?: boolean;
	storageAction: ReactNode;
	bindingVersion: number;
}) {
	const { t } = useTranslation();
	const result = useSWR(projectSettingsKey(project.name, workspaceEntryId, environment), () =>
		getProjectSettings(project.name, workspaceEntryId, environment),
	);
	const draft = useManifestDraftStore((state) => state.drafts[workspaceEntryId]);
	return (
		<div className="@container h-full min-h-0 overflow-y-auto p-4 ud-md:p-6">
			<div className="mx-auto flex h-full min-h-0 max-w-[1120px] flex-col gap-8">
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
								{!result.data && storageAction}
							</div>
						</AlertDescription>
					</Alert>
				)}
				{result.data && (
					<>
						<GeneralForm
							project={result.data.project}
							actions={
								<div className="flex flex-wrap items-center justify-end gap-2">
									{(readOnly || draft) && (
										<>
											{readOnly ? (
												<Badge variant="muted">
													<LockKeyhole />
													{t("projectInspector.manifestReadOnly")}
												</Badge>
											) : (
												<Badge variant="warning">{t("projectInspector.manifestDraft")}</Badge>
											)}
											{!readOnly && <ManifestSaveControl entryId={workspaceEntryId} />}
										</>
									)}
									{storageAction}
								</div>
							}
						/>
						<EnvironmentForm
							key={bindingVersion}
							project={result.data.project}
							environment={environment}
							workspaceEntryId={workspaceEntryId}
							readOnly={readOnly || result.isLoading || !!result.error}
						/>
					</>
				)}
			</div>
		</div>
	);
}
