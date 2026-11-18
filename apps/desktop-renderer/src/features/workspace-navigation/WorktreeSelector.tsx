import { useTranslation } from "react-i18next";
import { matchPath, useLocation, useNavigate, useParams } from "react-router-dom";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import { useWorkspace } from "./useWorkspace";
import {
	workspaceContext,
	workspaceKnowledgePath,
	workspacePath,
	workspaceVariablesPath,
} from "@/router/paths";

export function WorktreeSelector({ entryId }: { entryId: string }) {
	const { t } = useTranslation();
	const { registered, workspace } = useWorkspace(entryId);
	const { projectName } = useParams();
	const { pathname, search } = useLocation();
	const navigate = useNavigate();
	if (!registered?.worktrees?.length || !workspace?.worktreeId) return null;
	const onOverview =
		matchPath("/workspaces/:entryId", pathname) ||
		matchPath("/workspaces/:entryId/worktrees/:worktreeId", pathname);
	return (
		<Select
			value={workspace.worktreeId}
			onValueChange={(id) => {
				const context = workspaceContext(registered.entryId, id);
				if (onOverview) {
					navigate({ pathname: workspacePath(context), search });
					return;
				}
				const target = pathname.endsWith("/git/worktrees")
					? `${workspacePath(context)}/git/worktrees`
					: pathname.endsWith("/knowledge")
						? workspaceKnowledgePath(context)
						: workspaceVariablesPath(context, projectName);
				navigate({ pathname: target, search });
			}}
		>
			<SelectTrigger
				size="sm"
				className="w-[clamp(6rem,24vw,12rem)] text-xs [&>span:first-child]:truncate"
				aria-label={t("git.selectWorktree")}
			>
				<SelectValue />
			</SelectTrigger>
			<SelectContent>
				{registered.worktrees.map((tree) => (
					<SelectItem key={tree.id} value={tree.id}>
						{tree.branch || tree.root.split(/[/\\]/).pop() || t("git.detached")}
						{tree.main ? ` · ${t("git.main")}` : ""}
					</SelectItem>
				))}
			</SelectContent>
		</Select>
	);
}
