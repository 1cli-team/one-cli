import { FolderOpen, GitBranch, Plus, RefreshCw, Trash2 } from "lucide-react";
import { useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useLocation, useNavigate, useParams } from "react-router-dom";
import useSWR from "swr";
import { useSessionSWRConfig } from "@/lib/desktop/cache";
import { workspaceGroupsKey } from "@/api/workspace-groups";
import { preserveEnvironment } from "@/features/environment-context/environment";
import {
	addWorktree,
	type GitScope,
	getWorktrees,
	openWorktree,
	removeWorktree,
	worktreesKey,
	type AddWorktreeInput,
	type Worktree,
} from "@/api/git";
import { workspacesKey } from "@/api/workspaces";
import { Button } from "@/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import { ErrorNotice, PageHeader, StatePanel } from "@/components/ui/page-layout";
import { Skeleton } from "@/components/ui/skeleton";
import { useWorkspace } from "@/features/workspace-navigation/useWorkspace";
import { InitializeWorkspace } from "@/features/workspace-navigation/InitializeWorkspace";
import { workspacePath } from "@/router/paths";
import type { HttpError } from "@/types/api";
import { WorktreeForm } from "./WorktreeForm";

export function Worktrees() {
	const { entryId = "" } = useParams();
	const { workspace, registered, contextEntryId } = useWorkspace(entryId);
	const scope: GitScope = { kind: "workspace", id: contextEntryId };
	const { t } = useTranslation();
	const navigate = useNavigate();
	const { mutate } = useSessionSWRConfig();
	const { search } = useLocation();
	const repository = useSWR(worktreesKey(scope), () => getWorktrees(scope), {
		shouldRetryOnError: false,
	});
	const [creating, setCreating] = useState(false);
	const createContent = useRef<HTMLDivElement>(null);
	const [removing, setRemoving] = useState<Worktree | null>(null);
	const [busy, setBusy] = useState(false);
	const [error, setError] = useState("");
	const [notice, setNotice] = useState("");
	const readOnly = Boolean(repository.error);

	function message(error: unknown) {
		const text = (error as HttpError | Error)?.message;
		return text === "invalid_path" ? t("git.invalidPath") : text || t("git.failed");
	}

	async function act(action: () => Promise<void>) {
		setBusy(true);
		setError("");
		setNotice("");
		try {
			await action();
		} catch (error) {
			setError(message(error));
		} finally {
			setBusy(false);
		}
	}

	async function create(input: AddWorktreeInput) {
		await act(async () => {
			const result = await addWorktree(scope, input);
			setCreating(false);
			setNotice(t("git.created", { path: result.worktree.path }));
			if (result.warning) setError(result.warning);
			await Promise.all([repository.mutate(), mutate(workspacesKey), mutate(workspaceGroupsKey)]);
		});
	}

	async function remove(tree: Worktree) {
		await act(async () => {
			await removeWorktree(scope, tree.id);
			setRemoving(null);
			const updated = await mutate(workspacesKey);
			await mutate(workspaceGroupsKey);
			if (workspace && tree.workspaceRoot === workspace.root) {
				const remaining = updated?.workspaces?.find(
					(entry: { entryId: string }) => entry.entryId === registered?.entryId,
				);
				navigate(
					preserveEnvironment(
						remaining
							? `${workspacePath(remaining.entryId, remaining.worktreeId)}/git/worktrees`
							: "/",
						search,
					),
				);
				return;
			}
			setNotice(t("git.removed", { path: tree.path }));
			await repository.mutate();
		});
	}

	return (
		<div className="h-full overflow-y-auto p-4 ud-md:p-6">
			<div className="mx-auto flex max-w-[1120px] flex-col gap-6">
				<PageHeader
					title={
						repository.data?.name
							? t("git.repositoryWorktrees", { name: repository.data.name })
							: t("git.title")
					}
					actions={
						<>
							<Button
								variant="outline"
								disabled={busy || repository.isValidating}
								onClick={() => void repository.mutate()}
							>
								<RefreshCw />
								{t("git.refresh")}
							</Button>
							<Button
								disabled={busy || !repository.data || readOnly}
								onClick={() => {
									setError("");
									setCreating(true);
								}}
							>
								<Plus />
								{t("git.create")}
							</Button>
						</>
					}
				/>
				{(error || repository.error) && (
					<ErrorNotice>{error || message(repository.error)}</ErrorNotice>
				)}
				{notice && (
					<p role="status" className="break-all text-sm text-success-foreground">
						{notice}
					</p>
				)}
				{workspace?.status === "uninitialized" && <InitializeWorkspace workspace={workspace} />}
				{repository.isLoading && <Skeleton className="h-48 w-full" />}
				{repository.data?.remotes?.length ? (
					<div className="space-y-2 rounded-lg border border-border p-4">
						{repository.data.remotes.map((remote) => (
							<div key={remote.name} className="min-w-0 text-sm">
								<span className="font-medium">{remote.name}</span>
								<p className="break-all font-mono text-xs text-muted-foreground">
									{remote.fetchUrl}
								</p>
								{remote.pushUrl !== remote.fetchUrl && (
									<p className="break-all font-mono text-xs text-muted-foreground">
										{t("git.pushURL", { url: remote.pushUrl })}
									</p>
								)}
							</div>
						))}
					</div>
				) : null}
				{repository.data && (
					<div className="divide-y divide-border rounded-lg border border-border bg-card">
						{repository.data.worktrees.map((tree) => (
							<article
								key={tree.id}
								className="flex flex-wrap items-start justify-between gap-4 p-4"
							>
								<div className="min-w-0 flex-1 space-y-2">
									<div className="flex flex-wrap items-center gap-2">
										<GitBranch className="size-4 shrink-0 text-muted-foreground" />
										<h2 className="break-all font-medium">
											{tree.branch || t(tree.bare ? "git.bare" : "git.detached")}
										</h2>
										{tree.main && (
											<span className="text-xs text-muted-foreground">{t("git.main")}</span>
										)}
										{workspace && tree.workspaceRoot === workspace.root && (
											<span className="text-xs text-primary">{t("git.current")}</span>
										)}
									</div>
									<p className="break-all font-mono text-xs text-muted-foreground">{tree.path}</p>
									<div className="flex flex-wrap gap-x-3 gap-y-1 text-xs text-muted-foreground">
										{tree.head && <span className="font-mono">{tree.head.slice(0, 10)}</span>}
										{!tree.bare && !tree.missing && !tree.statusError && (
											<span>{t(tree.dirty ? "git.dirty" : "git.clean")}</span>
										)}
										{tree.missing && <span>{t("git.missing")}</span>}
										{tree.prunable && <span>{t("git.prunable")}</span>}
										{tree.locked && (
											<span>
												{t("git.locked")}
												{tree.lockReason && ` · ${tree.lockReason}`}
											</span>
										)}
									</div>
									{tree.statusError && (
										<p className="break-words text-sm text-error-foreground">{tree.statusError}</p>
									)}
								</div>
								<div className="flex flex-wrap gap-2">
									{tree.workspaceRoot && (
										<Button
											variant="outline"
											size="sm"
											disabled={busy || readOnly}
											onClick={() =>
												void act(async () => {
													const selected = await openWorktree(scope, tree.id);
													await mutate(workspacesKey);
													navigate(
														preserveEnvironment(
															workspacePath(selected.entryId, selected.worktreeId),
															search,
														),
													);
												})
											}
										>
											{t("git.openWorkspace")}
										</Button>
									)}
									{window.desktop && (
										<Button
											variant="outline"
											size="sm"
											disabled={busy || tree.missing}
											aria-label={t("git.openDirectoryFor", { path: tree.path })}
											onClick={() =>
												void act(async () => {
													await window.desktop!.openDirectory(tree.path);
												})
											}
										>
											<FolderOpen />
											{t("git.openDirectory")}
										</Button>
									)}
									{!tree.main && !tree.bare && (
										<Button
											variant="outline"
											size="sm"
											disabled={busy || readOnly || tree.locked}
											onClick={() => {
												setError("");
												setRemoving(tree);
											}}
										>
											<Trash2 />
											{t("git.remove")}
										</Button>
									)}
								</div>
							</article>
						))}
						{!repository.data.worktrees.length && (
							<StatePanel icon={GitBranch} title={t("git.empty")} />
						)}
					</div>
				)}
			</div>
			{creating && repository.data && (
				<Dialog
					open
					onOpenChange={(open) => {
						if (!open && !busy) setCreating(false);
					}}
				>
					<DialogContent ref={createContent} showCloseButton={!busy}>
						<DialogHeader>
							<DialogTitle>{t("git.create")}</DialogTitle>
							<DialogDescription>{t("git.createDescription")}</DialogDescription>
						</DialogHeader>
						<WorktreeForm
							repository={repository.data}
							container={createContent}
							busy={busy}
							error={error}
							onClose={() => setCreating(false)}
							onCreate={create}
						/>
					</DialogContent>
				</Dialog>
			)}
			<Dialog
				open={Boolean(removing)}
				onOpenChange={(open) => {
					if (!open && !busy) setRemoving(null);
				}}
			>
				<DialogContent showCloseButton={!busy}>
					<DialogHeader>
						<DialogTitle>{t("git.removeTitle")}</DialogTitle>
						<DialogDescription>{t("git.removeDescription")}</DialogDescription>
					</DialogHeader>
					<p className="break-all font-mono text-sm">{removing?.path}</p>
					{removing?.dirty && (
						<p className="text-sm text-warning-foreground">{t("git.removeDirty")}</p>
					)}
					{error && <ErrorNotice>{error}</ErrorNotice>}
					<DialogFooter>
						<Button variant="outline" disabled={busy} onClick={() => setRemoving(null)}>
							{t("git.cancel")}
						</Button>
						<Button
							variant="destructive"
							disabled={busy || Boolean(removing?.dirty)}
							onClick={() => removing && void remove(removing)}
						>
							{busy ? t("git.working") : t("git.remove")}
						</Button>
					</DialogFooter>
				</DialogContent>
			</Dialog>
		</div>
	);
}
