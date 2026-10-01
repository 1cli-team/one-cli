import { useState } from "react";
import { useTranslation } from "react-i18next";
import useSWR from "swr";
import { KeyRound, RefreshCw } from "lucide-react";
import { getWorkspaceEnvironment, workspaceEnvironmentKey } from "@/api/workspace";
import { getProjects, getSession, message, sessionKey } from "@/api/session";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { ErrorNotice } from "@/components/ui/page-layout";
import { Skeleton } from "@/components/ui/skeleton";
import { InfisicalBindingDialog } from "@/features/infisical-binding/InfisicalBindingDialog";

export function WorkspaceEnvironmentSettings({
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
	const [bindingOpen, setBindingOpen] = useState(false);
	const settings = useSWR(workspaceEnvironmentKey(workspaceEntryId, environment), () =>
		getWorkspaceEnvironment(workspaceEntryId, environment),
	);
	const session = useSWR(sessionKey, getSession);
	const projects = useSWR(
		session.data?.session.loggedIn ? "/infisical/projects" : null,
		getProjects,
	);
	const bound = !!settings.data?.projectId;
	const name =
		projects.data?.find((p) => p.id === settings.data?.projectId)?.name || settings.data?.projectId;
	return (
		<Card
			className="border-0 shadow-none"
			role="region"
			aria-labelledby="workspace-environment-title"
		>
			<CardContent className="space-y-5 p-0">
				<h2 id="workspace-environment-title" className="font-semibold">
					{t("overview.workspaceEnv.title")}
				</h2>
				{settings.isLoading ? (
					<Skeleton className="h-24" />
				) : settings.error ? (
					<ErrorNotice
						action={
							<Button variant="outline" onClick={() => void settings.mutate()}>
								<RefreshCw />
								{t("secrets.retry")}
							</Button>
						}
					>
						{message(settings.error)}
					</ErrorNotice>
				) : (
					<div className="space-y-4 rounded-md border p-4">
						<div className="flex flex-wrap items-center justify-between gap-3">
							<p className="font-medium">Infisical</p>
							<Button
								variant={bound ? "outline" : "default"}
								disabled={readOnly || !settings.data}
								onClick={() => setBindingOpen(true)}
							>
								<KeyRound />
								{t(bound ? "binding.change" : "binding.workspaceTitle")}
							</Button>
						</div>
						{bound ? (
							<div className="space-y-1 text-sm">
								<p className="break-all">{t("binding.current", { name })}</p>
								<p className="break-all text-muted-foreground">{settings.data?.siteUrl}</p>
								<p className="text-muted-foreground">{settings.data?.environments.join(" / ")}</p>
							</div>
						) : (
							<p className="text-sm text-muted-foreground">{t("binding.workspaceHint")}</p>
						)}
					</div>
				)}
				<p className="text-xs text-muted-foreground">{t("binding.workspaceStorage")}</p>
				<InfisicalBindingDialog
					key={`${workspaceEntryId}:${session.data?.session.siteUrl}:${session.data?.session.userId}:${session.data?.session.organizationId}`}
					open={bindingOpen}
					onOpenChange={setBindingOpen}
					scope="workspace"
					workspaceEntryId={workspaceEntryId}
					environment={environment}
					readOnly={readOnly}
				/>
			</CardContent>
		</Card>
	);
}
