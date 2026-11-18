import { FolderOpen, GitBranch, GitFork, Plus, RefreshCw, SearchX, Trash2 } from "lucide-react";
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
	type GitRemote,
	getWorktrees,
	openWorktree,
	removeWorktree,
	worktreesKey,
	type AddWorktreeInput,
	type Worktree,
} from "@/api/git";
import { workspacesKey } from "@/api/workspaces";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import {
	ErrorNotice,
	PageContent,
	PageHeader,
	PageToolbar,
	SearchInput,
	SectionHeading,
	StatePanel,
} from "@/components/ui/page-layout";
import { Skeleton } from "@/components/ui/skeleton";
import { useWorkspace } from "@/features/workspace-navigation/useWorkspace";
import { InitializeWorkspace } from "@/features/workspace-navigation/InitializeWorkspace";
import { workspacePath } from "@/router/paths";
import type { HttpError } from "@/types/api";
import { WorktreeForm } from "./WorktreeForm";
import { GitOriginDialog } from "./GitOriginDialog";

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
	const [originDraft, setOriginDraft] = useState<{ scope: GitScope; origin?: GitRemote }>();
	const createContent = useRef<HTMLDivElement>(null);
	const [removing, setRemoving] = useState<Worktree | null>(null);
	const [busy, setBusy] = useState(false);
	const [error, setError] = useState("");
	const [notice, setNotice] = useState("");
	const [filter, setFilter] = useState("");
	const readOnly = Boolean(repository.error);
	const origin = repository.data?.remotes.find((remote) => remote.name === "origin");
	const query = filter.trim().toLowerCase();
	const worktrees = (repository.data?.worktrees ?? []).filter(
		(tree) => tree.path.toLowerCase().includes(query) || tree.branch?.toLowerCase().includes(query),
	);

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
		<PageContent scroll>
			<PageHeader
				title={t("git.title")}
				description={repository.data?.name}
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
			{repository.data && (
				<PageToolbar className="justify-between">
					<SearchInput
						groupClassName="max-w-sm"
						aria-label={t("git.searchWorktrees")}
						placeholder={t("git.searchWorktrees")}
						value={filter}
						onChange={(event) => setFilter(event.target.value)}
					/>
					<Button
						variant="outline"
						disabled={busy || readOnly}
						onClick={() => {
							setNotice("");
							setOriginDraft({ scope, origin });
						}}
					>
						<GitFork />
						{t(origin ? "git.editOrigin" : "git.bindOrigin")}
					</Button>
				</PageToolbar>
			)}
			{repository.data && (
				<div className="divide-y divide-border rounded-lg border border-border bg-card">
					{worktrees.map((tree) => (
						<article key={tree.id} className="flex flex-wrap items-start justify-between gap-4 p-4">
							<div className="min-w-0 flex-1 space-y-2">
								<div className="flex flex-wrap items-center gap-2">
									<GitBranch className="size-4 shrink-0 text-muted-foreground" />
									<h2 className="break-all font-medium">
										{tree.branch || t(tree.bare ? "git.bare" : "git.detached")}
									</h2>
									{tree.main && <Badge variant="muted">{t("git.main")}</Badge>}
									{workspace && tree.workspaceRoot === workspace.root && (
										<Badge variant="secondary">{t("git.current")}</Badge>
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
					{worktrees.length === 0 &&
						(repository.data.worktrees.length > 0 ? (
							<StatePanel icon={SearchX} title={t("git.noMatchingWorktrees")}>
								<Button variant="outline" onClick={() => setFilter("")}>
									{t("workspaces.home.clearFilters")}
								</Button>
							</StatePanel>
						) : (
							<StatePanel icon={GitBranch} title={t("git.empty")} />
						))}
				</div>
			)}
			{repository.data && (
				<section
					className="space-y-3 rounded-lg border border-border bg-card p-4"
					aria-label={t("git.remoteRepositories")}
				>
					<SectionHeading icon={GitFork} title={t("git.remoteRepositories")} />
					{!origin && <p className="text-sm text-muted-foreground">{t("git.originMissing")}</p>}
					{repository.data.remotes.map((remote) => (
						<div key={remote.name} className="min-w-0 text-sm">
							<span className="font-medium">{remote.name}</span>
							<p className="font-mono text-xs text-muted-foreground wrap-anywhere">
								{remote.fetchUrl}
							</p>
							{remote.pushUrl !== remote.fetchUrl && (
								<p className="font-mono text-xs text-muted-foreground wrap-anywhere">
									{t("git.pushURL", { url: remote.pushUrl })}
								</p>
							)}
						</div>
					))}
				</section>
			)}
			{originDraft && (
				<GitOriginDialog
					scope={originDraft.scope}
					origin={originDraft.origin}
					onClose={() => setOriginDraft(undefined)}
					onSaved={(saved) => {
						setNotice(t("git.originSaved"));
						void mutate(worktreesKey(originDraft.scope), (current: typeof repository.data) => {
							if (!current) return current;
							const remotes = current.remotes.some((remote) => remote.name === "origin")
								? current.remotes.map((remote) => (remote.name === "origin" ? saved : remote))
								: [...current.remotes, saved];
							return { ...current, remotes };
						});
					}}
				/>
			)}
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
		</PageContent>
	);
}
