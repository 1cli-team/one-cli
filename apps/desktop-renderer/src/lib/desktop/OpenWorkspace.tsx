import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";
import { FolderOpen } from "lucide-react";
import { Button } from "@/components/ui/button";
import { AddWorkspaceDialog } from "@/features/workspace-navigation/AddWorkspaceDialog";
import { workspacePath } from "@/router/paths";
import { useSessionSWRConfig } from "./cache";

export function OpenWorkspace() {
	const { t } = useTranslation();
	const [open, setOpen] = useState(false);
	const navigate = useNavigate();
	const { mutate } = useSessionSWRConfig();
	return (
		<>
			<Button onClick={() => setOpen(true)}>
				<FolderOpen />
				{t("desktop.openWorkspace")}
			</Button>
			{open && (
				<AddWorkspaceDialog
					onClose={() => setOpen(false)}
					onSaved={(workspace) => {
						setOpen(false);
						void mutate("/workspaces");
						navigate(workspacePath(workspace.entryId, workspace.worktreeId));
					}}
				/>
			)}
		</>
	);
}
