import { useTranslation } from "react-i18next";
import { useLocation, useNavigate, useParams } from "react-router-dom";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import { useWorkspace } from "./useWorkspace";
import { projectPath, workspaceContext, workspacePath } from "@/router/paths";

export function WorktreeSelector({ entryId }: { entryId: string }) {
	const { t } = useTranslation();
	const { registered, workspace } = useWorkspace(entryId);
	const { projectName } = useParams();
	const { pathname, search } = useLocation();
	const navigate = useNavigate();
	if (!registered?.worktrees?.length || !workspace?.worktreeId) return null;
	return (
		<Select
			value={workspace.worktreeId}
			onValueChange={(id) => {
				const context = workspaceContext(registered.entryId, id);
				let target = workspacePath(context);
				if (pathname.endsWith("/git/worktrees")) target += "/git/worktrees";
				else if (projectName) target = projectPath(context, projectName);
				navigate({ pathname: target, search });
			}}
		>
			<SelectTrigger className="w-[min(14rem,35vw)]" aria-label={t("git.selectWorktree")}>
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
