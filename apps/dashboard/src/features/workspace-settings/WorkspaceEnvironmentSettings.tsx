import { useTranslation } from "react-i18next";
import useSWR from "swr";
import { getWorkspaceEnvironment, workspaceEnvironmentKey } from "@/api/workspace";
import { getProjects, getSession, message, sessionKey } from "@/api/session";
import { Card, CardContent } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import {
	manifestDraftKey,
	useManifestDraftStore,
} from "@/features/manifest-draft/manifest-draft-store";
import { SessionStatus } from "@/features/infisical-session/AccountSettings";
import type { WorkspaceEnvironmentPatch } from "@/types/api";
export function WorkspaceEnvironmentSettings({
	currentBackend,
	environment,
	workspaceEntryId,
	readOnly = false,
}: {
	currentBackend?: string;
	environment: string;
	workspaceEntryId?: string;
	readOnly?: boolean;
}) {
	const { t } = useTranslation();
	const settings = useSWR(workspaceEnvironmentKey(workspaceEntryId, environment), () =>
		getWorkspaceEnvironment(workspaceEntryId, environment),
	);
	const session = useSWR(sessionKey, getSession);
	const projects = useSWR(
		session.data?.session.loggedIn ? "/infisical/projects" : null,
		getProjects,
	);
	const staged = useManifestDraftStore(
		(s) => s.drafts[manifestDraftKey(workspaceEntryId)]?.workspace?.environment,
	);
	const stage = useManifestDraftStore((s) => s.stageWorkspaceSection);
	const initial: WorkspaceEnvironmentPatch = {
		backend: settings.data?.backend ?? currentBackend ?? "dotenv",
		...(settings.data?.projectId
			? {
					projectId: settings.data.projectId,
					projectName: settings.data.projectName,
					siteUrl: settings.data.siteUrl,
				}
			: {}),
	};
	const value = staged ?? initial;
	function change(next: WorkspaceEnvironmentPatch) {
		if (!settings.data || readOnly) return;
		stage({
			entryId: workspaceEntryId,
			revision: settings.data.revision,
			section: "environment",
			initial,
			next,
			labels: {
				backend: "overview.workspaceEnv.backend",
				projectId: "global.project",
				projectName: "global.project",
				siteUrl: "session.site",
			},
		});
	}
	return (
		<Card role="region" aria-labelledby="workspace-environment-title">
			<CardContent className="space-y-4 p-4">
				{staged ? (
					<p className="text-xs text-warning-foreground">
						{t("overview.workspaceEnv.backendPending")}
					</p>
				) : null}
				<h2 id="workspace-environment-title" className="font-semibold">
					{t("overview.workspaceEnv.title")}
				</h2>
				<div data-testid="workspace-backend-settings" className="grid gap-4 sm:grid-cols-2">
					<div className="space-y-2">
						<Label>{t("overview.workspaceEnv.backend")}</Label>
						<Select
							value={value.backend}
							disabled={readOnly || !settings.data}
							onValueChange={(backend) =>
								change(backend === "dotenv" ? { backend } : { ...initial, backend })
							}
						>
							<SelectTrigger aria-label={t("overview.workspaceEnv.backend")}>
								<SelectValue />
							</SelectTrigger>
							<SelectContent>
								<SelectItem value="dotenv">dotenv</SelectItem>
								<SelectItem value="infisical">Infisical</SelectItem>
							</SelectContent>
						</Select>
					</div>
					{value.backend === "infisical" ? (
						<div className="space-y-2">
							<Label>{t("global.project")}</Label>
							<Select
								value={value.projectId ?? ""}
								disabled={readOnly || !session.data?.session.loggedIn}
								onValueChange={(id) => {
									const p = projects.data?.find((p) => p.id === id);
									if (p)
										change({
											...value,
											projectId: p.id,
											projectName: p.name,
											siteUrl: session.data?.session.siteUrl,
										});
								}}
							>
								<SelectTrigger aria-label={t("global.project")}>
									<SelectValue placeholder={value.projectName || t("global.selectProject")} />
								</SelectTrigger>
								<SelectContent>
									{projects.data?.map((p) => (
										<SelectItem key={p.id} value={p.id}>
											{p.name}
										</SelectItem>
									))}
								</SelectContent>
							</Select>
							<SessionStatus />
						</div>
					) : null}
				</div>
				<p className="text-xs text-muted-foreground">{t("overview.workspaceEnv.backendSource")}</p>
				{settings.error || projects.error ? (
					<p role="alert">{message(settings.error || projects.error)}</p>
				) : null}
			</CardContent>
		</Card>
	);
}
