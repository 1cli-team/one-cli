import { useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import useSWR from "swr";
import {
	addWorktree,
	getWorktrees,
	worktreesKey,
	type AddWorktreeInput,
	type GitScope,
	type WorktreeChange,
} from "@/api/git";
import { workspaceGroupsKey } from "@/api/workspace-groups";
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
import { Label } from "@/components/ui/label";
import { ErrorNotice } from "@/components/ui/page-layout";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import { WorktreeForm } from "@/features/git/WorktreeForm";
import { useWorkspaceGroups } from "@/features/workspace-groups/useWorkspaceGroups";
import { useSessionSWRConfig } from "@/lib/desktop/cache";
import { errorText, type Issue, type Scope } from "./api";

export function CreateIssueWorktreeDialog({
	scope,
	issue,
	onClose,
}: {
	scope: Scope;
	issue: Issue;
	onClose(): void;
}) {
	const { t } = useTranslation();
	const { mutate } = useSessionSWRConfig();
	const registry = useWorkspaceGroups();
	const ids =
		scope.kind === "group"
			? registry.data?.groups.find((group) => group.id === scope.id)?.workspaceIds || []
			: [scope.id.split("~")[0]];
	const repositories =
		registry.data?.workspaces.filter(
			(workspace) =>
				(ids.includes(workspace.entryId) ||
					workspace.aliases?.some((alias) => ids.includes(alias.entryId))) &&
				workspace.gitCommonDir &&
				["ready", "uninitialized"].includes(workspace.status),
		) || [];
	const [workspaceId, setWorkspaceId] = useState("");
	const selected =
		repositories.find((repository) => repository.entryId === workspaceId) ||
		(repositories.length === 1 ? repositories[0] : undefined);
	const gitScope: GitScope = {
		kind: "workspace",
		id: selected ? (scope.kind === "workspace" ? scope.id : selected.entryId) : "",
	};
	const repository = useSWR(
		selected ? worktreesKey(gitScope) : null,
		() => getWorktrees(gitScope),
		{ shouldRetryOnError: false },
	);
	const slug = Array.from(
		issue.title
			.normalize("NFKC")
			.toLowerCase()
			.replace(/[^\p{L}\p{N}]+/gu, "-"),
	)
		.slice(0, 80)
		.join("")
		.replace(/^-+|-+$/g, "");
	const defaultBranch = `${issue.identifier.toLowerCase()}${slug ? `-${slug}` : ""}`;
	const [busy, setBusy] = useState(false);
	const [error, setError] = useState("");
	const [created, setCreated] = useState<WorktreeChange | null>(null);
	const contentRef = useRef<HTMLDivElement>(null);

	async function create(input: AddWorktreeInput) {
		if (!selected || busy || created) return;
		setBusy(true);
		setError("");
		try {
			setCreated(await addWorktree(gitScope, input));
			void Promise.allSettled([
				repository.mutate(),
				mutate(workspacesKey),
				mutate(workspaceGroupsKey),
			]);
		} catch (error) {
			setError(errorText(error));
		} finally {
			setBusy(false);
		}
	}

	return (
		<Dialog
			open
			onOpenChange={(open) => {
				if (!open && !busy) onClose();
			}}
		>
			<DialogContent ref={contentRef} showCloseButton={!busy}>
				<DialogHeader>
					<DialogTitle>{t("git.create")}</DialogTitle>
					<DialogDescription>
						{issue.identifier} · {issue.title}
					</DialogDescription>
				</DialogHeader>
				{created ? (
					<>
						<p role="status" className="break-all text-sm text-success-foreground">
							{t("git.created", { path: created.worktree.path })}
						</p>
						{created.warning && <ErrorNotice>{created.warning}</ErrorNotice>}
						<DialogFooter>
							<Button onClick={onClose}>{t("form.close")}</Button>
						</DialogFooter>
					</>
				) : (
					<>
						<div className="space-y-2">
							<Label htmlFor="issue-worktree-repository">{t("linear.worktreeRepository")}</Label>
							<Select
								value={selected?.entryId || ""}
								disabled={busy || !repositories.length}
								onValueChange={(id) => {
									setWorkspaceId(id);
									setError("");
								}}
							>
								<SelectTrigger id="issue-worktree-repository">
									<SelectValue placeholder={t("linear.worktreeChooseRepository")} />
								</SelectTrigger>
								<SelectContent>
									{repositories.map((repository) => (
										<SelectItem key={repository.entryId} value={repository.entryId}>
											{repository.name}
										</SelectItem>
									))}
								</SelectContent>
							</Select>
							{selected && (
								<p className="break-all text-xs text-muted-foreground">{selected.root}</p>
							)}
							{registry.isLoading && (
								<p className="text-sm text-muted-foreground" role="status">
									{t("linear.loading")}
								</p>
							)}
							{registry.data && !repositories.length && (
								<p className="text-sm text-muted-foreground">{t("linear.worktreeNoRepository")}</p>
							)}
						</div>
						{(registry.error || repository.error) && (
							<ErrorNotice>{errorText(registry.error || repository.error)}</ErrorNotice>
						)}
						{repository.isLoading && (
							<p role="status" className="text-sm text-muted-foreground">
								{t("linear.loading")}
							</p>
						)}
						{repository.data && (
							<WorktreeForm
								key={gitScope.id}
								repository={repository.data}
								container={contentRef}
								defaultBranch={defaultBranch}
								busy={busy}
								error={error}
								onClose={onClose}
								onCreate={create}
							/>
						)}
					</>
				)}
			</DialogContent>
		</Dialog>
	);
}
