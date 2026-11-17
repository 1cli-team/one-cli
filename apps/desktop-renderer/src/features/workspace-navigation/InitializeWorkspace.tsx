import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";
import { useSWRConfig } from "swr";
import { Button } from "@/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogHeader,
	DialogTitle,
	DialogDescription,
	DialogFooter,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ErrorNotice } from "@/components/ui/page-layout";
import { initializeWorkspace, workspacesKey } from "@/api/workspaces";
import { workspacePath } from "@/router/paths";
import type { WorkspaceRegistryEntry } from "@/types/api";

export function InitializeWorkspace({ workspace }: { workspace: WorkspaceRegistryEntry }) {
	const { t } = useTranslation();
	const { mutate } = useSWRConfig();
	const navigate = useNavigate();
	const [open, setOpen] = useState(false);
	const [name, setName] = useState(workspace.name);
	const [busy, setBusy] = useState(false);
	const [error, setError] = useState("");
	async function initialize() {
		setBusy(true);
		setError("");
		try {
			const result = await initializeWorkspace(workspace.entryId, name);
			await mutate(workspacesKey);
			setOpen(false);
			navigate(workspacePath(result.entryId, result.worktreeId));
		} catch (error) {
			setError((error as Error).message);
		} finally {
			setBusy(false);
		}
	}
	return (
		<>
			<Button onClick={() => setOpen(true)}>{t("workspaces.initialize")}</Button>
			<Dialog
				open={open}
				onOpenChange={(value) => {
					if (!busy) setOpen(value);
				}}
			>
				<DialogContent showCloseButton={!busy}>
					<DialogHeader>
						<DialogTitle>{t("workspaces.initialize")}</DialogTitle>
						<DialogDescription>
							{t("workspaces.initializeDescription", { path: workspace.root })}
						</DialogDescription>
					</DialogHeader>
					<form
						onSubmit={(event) => {
							event.preventDefault();
							void initialize();
						}}
						className="space-y-4"
					>
						<div className="space-y-2">
							<Label htmlFor="workspace-name">{t("workspaces.initializeName")}</Label>
							<Input
								id="workspace-name"
								value={name}
								onChange={(event) => setName(event.target.value)}
								required
								disabled={busy}
							/>
						</div>
						{error && <ErrorNotice>{error}</ErrorNotice>}
						<DialogFooter>
							<Button
								type="button"
								variant="outline"
								disabled={busy}
								onClick={() => setOpen(false)}
							>
								{t("git.cancel")}
							</Button>
							<Button type="submit" disabled={busy}>
								{busy ? t("git.working") : t("workspaces.initialize")}
							</Button>
						</DialogFooter>
					</form>
				</DialogContent>
			</Dialog>
		</>
	);
}
