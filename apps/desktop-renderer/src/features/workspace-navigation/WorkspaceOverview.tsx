import { GitBranch } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { PageContent, PageHeader } from "@/components/ui/page-layout";
import { EnvironmentLink } from "@/features/environment-context/EnvironmentLink";
import { workspacePath } from "@/router/paths";
import type { WorkspaceRegistryEntry } from "@/types/api";
import { InitializeWorkspace } from "./InitializeWorkspace";
import { isGitRepositoryWithoutManifest, workspaceDisplayStatus } from "./workspace-status";

export function WorkspaceOverview({ workspace }: { workspace: WorkspaceRegistryEntry }) {
	const { t } = useTranslation();
	const worktree = workspace.worktrees?.find((tree) => tree.id === workspace.worktreeId);
	const canInitialize = workspace.status === "uninitialized";
	const status = workspaceDisplayStatus(workspace);
	const fields = [
		{ label: t("workspaces.overview.directory"), value: workspace.root },
		...(worktree ? [{ label: t("git.branch"), value: worktree.branch || t("git.detached") }] : []),
	];
	if (!isGitRepositoryWithoutManifest(workspace)) {
		fields.push({ label: t("workspaces.overview.modules"), value: String(workspace.projectCount) });
	}
	return (
		<PageContent scroll>
			<PageHeader
				title={t("navigation.overview")}
				description={workspace.name}
				status={
					<Badge variant={status === "ready" ? "success" : "warning"}>
						{t(`workspaces.status.${status}`)}
					</Badge>
				}
				actions={
					workspace.gitCommonDir || canInitialize ? (
						<>
							{workspace.gitCommonDir && (
								<Button variant="outline" asChild>
									<EnvironmentLink to={`${workspacePath(workspace.entryId)}/git/worktrees`}>
										<GitBranch data-icon="inline-start" />
										{t("git.title")}
									</EnvironmentLink>
								</Button>
							)}
							{canInitialize && <InitializeWorkspace workspace={workspace} />}
						</>
					) : undefined
				}
			/>
			<dl className="divide-y divide-border rounded-lg border border-border bg-card">
				{fields.map(({ label, value }) => (
					<div
						key={label}
						className="grid min-w-0 gap-1 px-4 py-3 @min-[32rem]:grid-cols-[9rem_minmax(0,1fr)] @min-[32rem]:gap-4"
					>
						<dt className="text-sm text-muted-foreground">{label}</dt>
						<dd className="min-w-0 font-mono text-sm wrap-anywhere">{value}</dd>
					</div>
				))}
			</dl>
		</PageContent>
	);
}
