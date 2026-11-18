import { useTranslation } from "react-i18next";
import useSWR from "swr";
import { getWorkspaces, workspacesKey } from "@/api/workspaces";
import { Button } from "@/components/ui/button";
import { EnvironmentLink } from "@/features/environment-context/EnvironmentLink";
import { workspaceKnowledgePath } from "@/router/paths";
import type { SessionLink } from "./api";

export function GatewaySessionLink({ session }: { session: SessionLink }) {
	const { t } = useTranslation();
	const { data } = useSWR(workspacesKey, getWorkspaces);
	const workspace = data?.workspaces.find(
		(entry) => entry.id === session.workspaceId && entry.status === "ready",
	);
	if (!workspace) {
		return (
			<p className="text-sm text-muted-foreground">
				{t("openviking.viewSession")} <code className="break-all">{session.id}</code>
			</p>
		);
	}
	const search = new URLSearchParams({ section: "sessions", session: session.id });
	return (
		<Button asChild variant="link" size="sm" className="w-fit px-0">
			<EnvironmentLink to={workspaceKnowledgePath(workspace.entryId) + "?" + search}>
				{t("openviking.viewSession")}
			</EnvironmentLink>
		</Button>
	);
}
