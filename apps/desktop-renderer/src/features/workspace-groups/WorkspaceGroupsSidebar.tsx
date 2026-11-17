import { FolderGit2, FolderKanban } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import {
	EnvironmentLink,
	EnvironmentNavLink,
} from "@/features/environment-context/EnvironmentLink";
import { groupPath, groupWorkspacePath, workspacePath } from "@/router/paths";
import { useWorkspaceGroups } from "./useWorkspaceGroups";

export function WorkspaceGroupsSidebar() {
	const { t } = useTranslation();
	const result = useWorkspaceGroups();
	const data = result.data;
	function members(ids: string[], groupId?: string) {
		return ids.map((id) => {
			const workspace = data?.workspaces.find((item) => item.entryId === id);
			if (!workspace) return null;
			return (
				<Button key={id} variant="navigation" size="navigation-compact" asChild>
					<EnvironmentLink
						to={groupId ? groupWorkspacePath(groupId, id) : workspacePath(id)}
						title={`${workspace.name}\n${workspace.root}`}
					>
						<FolderGit2 />
						<span className="min-w-0 flex-1 truncate">{workspace.name}</span>
					</EnvironmentLink>
				</Button>
			);
		});
	}
	return (
		<nav
			className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto px-3 py-3"
			aria-label={t("groups.title")}
		>
			{result.isLoading && <Skeleton className="h-20" />}
			{result.error && (
				<Button variant="outline" onClick={() => void result.mutate()}>
					{t("workspaces.retry")}
				</Button>
			)}
			{data?.projects.map((project) => (
				<section key={project.id} className="flex flex-col gap-1">
					{project.kind === "workspace" ? (
						members(project.workspaceIds)
					) : (
						<>
							<Button variant="navigation" size="navigation-compact" asChild>
								<EnvironmentNavLink to={groupPath(project.id)}>
									<FolderKanban />
									<span className="min-w-0 flex-1 truncate">{project.name}</span>
									<span className="text-xs tabular-nums text-muted-foreground">
										{project.workspaceIds.length}
									</span>
								</EnvironmentNavLink>
							</Button>
							<div className="ml-3 flex flex-col gap-1 border-l border-border pl-2">
								{members(project.workspaceIds, project.id)}
							</div>
						</>
					)}
				</section>
			))}

			<Button variant="navigation" size="navigation-compact" asChild>
				<EnvironmentNavLink to="/workspaces" end>
					<FolderGit2 />
					{t("groups.allWorkspaces")}
				</EnvironmentNavLink>
			</Button>
		</nav>
	);
}
