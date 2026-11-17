import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";
import { FolderOpen } from "lucide-react";
import { Button } from "@/components/ui/button";
import { AddWorkspaceDialog } from "@/features/workspace-navigation/AddWorkspaceDialog";
import { addGroupMembers, workspaceGroupsKey } from "@/api/workspace-groups";
import { groupWorkspacePath, workspacePath } from "@/router/paths";
import { useSessionSWRConfig } from "./cache";

export function OpenWorkspace({ groupId, label }: { groupId?: string; label?: string }) {
	const { t } = useTranslation();
	const [open, setOpen] = useState(false);
	const navigate = useNavigate();
	const { mutate } = useSessionSWRConfig();
	return (
		<>
			<Button onClick={() => setOpen(true)}>
				<FolderOpen />
				{label ?? t("desktop.openWorkspace")}
			</Button>
			{open && (
				<AddWorkspaceDialog
					onClose={() => setOpen(false)}
					onSaved={async (workspace) => {
						if (groupId) await addGroupMembers(groupId, [workspace.entryId]);
						setOpen(false);
						await Promise.all([
							mutate("/workspaces", undefined, { revalidate: true, throwOnError: false }),
							mutate(workspaceGroupsKey, undefined, { revalidate: true, throwOnError: false }),
						]);
						navigate(
							groupId
								? groupWorkspacePath(groupId, workspace.entryId, workspace.worktreeId)
								: workspacePath(workspace.entryId, workspace.worktreeId),
						);
					}}
				/>
			)}
		</>
	);
}
