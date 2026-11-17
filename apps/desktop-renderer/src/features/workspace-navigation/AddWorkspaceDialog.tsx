import { FolderOpen } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { cloneRepository } from "@/api/git";
import { openLocalWorkspace, relocateWorkspace } from "@/api/workspaces";
import { Button } from "@/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ErrorNotice } from "@/components/ui/page-layout";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import type { WorkspaceRegistryEntry } from "@/types/api";

function errorMessage(error: unknown) {
	return (error as Error)?.message || String(error);
}

export function AddWorkspaceDialog({
	workspace,
	onClose,
	onSaved,
}: {
	workspace?: WorkspaceRegistryEntry;
	onClose(): void;
	onSaved(entry: WorkspaceRegistryEntry): void;
}) {
	const { t } = useTranslation();
	const [mode, setMode] = useState("local");
	const [path, setPath] = useState(workspace?.root ?? "");
	const [url, setURL] = useState("");
	const [busy, setBusy] = useState(false);
	const [error, setError] = useState("");

	async function submit() {
		setBusy(true);
		setError("");
		try {
			if (workspace) {
				onSaved(await relocateWorkspace(workspace.entryId, path));
				return;
			}
			if (mode === "local") {
				onSaved(await openLocalWorkspace(path));
				return;
			}
			const result = await cloneRepository(url, path);
			if (result.workspace) onSaved(result.workspace);
			else {
				setMode("local");
				setPath(result.path);
				setError(
					result.warning || t("git.cloneAssociationWarning", { path: result.path, error: "" }),
				);
			}
		} catch (error) {
			setError(errorMessage(error));
		} finally {
			setBusy(false);
		}
	}

	async function chooseDirectory() {
		try {
			const selected = await window.desktop?.chooseWorkspace();
			if (selected) setPath(selected);
		} catch (error) {
			setError(errorMessage(error));
		}
	}

	return (
		<Dialog
			open
			onOpenChange={(open) => {
				if (!open && !busy) onClose();
			}}
		>
			<DialogContent showCloseButton={!busy}>
				<DialogHeader>
					<DialogTitle>{t(workspace ? "git.locateRepository" : "git.linkRepository")}</DialogTitle>
					<DialogDescription>
						{t(workspace ? "git.locateRepositoryDescription" : "git.linkRepositoryDescription")}
					</DialogDescription>
				</DialogHeader>
				<form
					className="space-y-4"
					onSubmit={(event) => {
						event.preventDefault();
						void submit();
					}}
				>
					{!workspace && (
						<div className="space-y-2">
							<Label htmlFor="repository-mode">{t("git.repositorySource")}</Label>
							<Select value={mode} onValueChange={setMode} disabled={busy}>
								<SelectTrigger id="repository-mode">
									<SelectValue />
								</SelectTrigger>
								<SelectContent>
									<SelectItem value="local">{t("git.localRepository")}</SelectItem>
									<SelectItem value="clone">{t("git.cloneRepository")}</SelectItem>
								</SelectContent>
							</Select>
						</div>
					)}
					{mode === "clone" && (
						<div className="space-y-2">
							<Label htmlFor="repository-url">{t("git.repositoryURL")}</Label>
							<Input
								id="repository-url"
								value={url}
								onChange={(event) => setURL(event.target.value)}
								disabled={busy}
								required
								placeholder="git@github.com:owner/repository.git"
							/>
							<p className="text-xs text-muted-foreground">{t("git.cloneAuthentication")}</p>
						</div>
					)}
					<div className="space-y-2">
						<Label htmlFor="repository-path">
							{t(mode === "clone" ? "git.cloneDestination" : "git.repositoryDirectory")}
						</Label>
						<div className="flex gap-2">
							<Input
								id="repository-path"
								value={path}
								onChange={(event) => setPath(event.target.value)}
								disabled={busy}
								required
							/>
							{mode === "local" && window.desktop && (
								<Button
									type="button"
									variant="outline"
									disabled={busy}
									onClick={() => void chooseDirectory()}
								>
									<FolderOpen />
									{t("git.browse")}
								</Button>
							)}
						</div>
					</div>
					{error && <ErrorNotice>{error}</ErrorNotice>}
					{busy && mode === "clone" && (
						<p role="status" className="text-sm text-muted-foreground">
							{t("git.cloning")}
						</p>
					)}
					<DialogFooter>
						<Button type="button" variant="outline" disabled={busy} onClick={onClose}>
							{t("git.cancel")}
						</Button>
						<Button type="submit" disabled={busy}>
							{busy
								? t("git.working")
								: t(mode === "clone" ? "git.cloneRepository" : "git.saveRepository")}
						</Button>
					</DialogFooter>
				</form>
			</DialogContent>
		</Dialog>
	);
}
