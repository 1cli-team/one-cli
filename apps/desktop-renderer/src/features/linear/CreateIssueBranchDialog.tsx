import { useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import useSWR from "swr";
import { Check, GitBranchPlus } from "lucide-react";
import {
	createBranch,
	getWorktrees,
	worktreesKey,
	type BranchResult,
	type GitScope,
} from "@/api/git";
import { workspaceGroupsKey } from "@/api/workspace-groups";
import { workspacesKey } from "@/api/workspaces";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
	Combobox,
	ComboboxContent,
	ComboboxInput,
	ComboboxItem,
	ComboboxList,
} from "@/components/ui/combobox";
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
import { useWorkspaceGroups } from "@/features/workspace-groups/useWorkspaceGroups";
import { useSessionSWRConfig } from "@/lib/desktop/cache";
import { errorText, type Issue, type Scope } from "./api";

export function CreateIssueBranchDialog({
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
	const [branch, setBranch] = useState(
		`${issue.identifier.toLowerCase()}${slug ? `-${slug}` : ""}`,
	);
	const [startPoint, setStartPoint] = useState("HEAD");
	const [checkout, setCheckout] = useState(false);
	const [busy, setBusy] = useState(false);
	const [error, setError] = useState("");
	const [created, setCreated] = useState<BranchResult | null>(null);
	const contentRef = useRef<HTMLDivElement>(null);
	const startPoints = [
		"HEAD",
		...(repository.data?.branches || []).filter((name) => name !== "HEAD"),
	];
	const exists = repository.data?.branches.includes(branch.trim());
	async function submit() {
		if (!selected || !repository.data || busy || created) return;
		setBusy(true);
		setError("");
		try {
			const result = await createBranch(gitScope, {
				branch: branch.trim(),
				startPoint: startPoint.trim() || "HEAD",
				checkout,
			});
			setCreated(result);
			void Promise.allSettled([
				mutate(worktreesKey(gitScope)),
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
					<DialogTitle>{t("linear.createBranch")}</DialogTitle>
					<DialogDescription>
						{issue.identifier} · {issue.title}
					</DialogDescription>
				</DialogHeader>
				{created ? (
					<div className="space-y-4">
						<div
							className="flex items-start gap-3 rounded-md border border-border bg-muted/50 p-4"
							role="status"
						>
							<Check className="mt-0.5 size-4 shrink-0 text-success-foreground" />
							<div className="min-w-0 space-y-1">
								<p className="text-sm font-medium">
									{t(
										created.checkedOut ? "linear.branchCreatedAndSwitched" : "linear.branchCreated",
									)}
								</p>
								<p className="break-all font-mono text-sm">{created.branch}</p>
								<p className="break-all text-xs text-muted-foreground">
									{selected?.name} · {selected?.root}
								</p>
							</div>
						</div>
						{created.warning && <ErrorNotice>{created.warning}</ErrorNotice>}
						<DialogFooter>
							<Button onClick={onClose}>{t("linear.branchDone")}</Button>
						</DialogFooter>
					</div>
				) : (
					<form
						className="space-y-4"
						onSubmit={(event) => {
							event.preventDefault();
							void submit();
						}}
					>
						<div className="space-y-2">
							<Label htmlFor="issue-branch-repository">{t("linear.branchRepository")}</Label>
							<Select
								value={selected?.entryId || ""}
								disabled={busy || !repositories.length}
								onValueChange={(id) => {
									setWorkspaceId(id);
									setStartPoint("HEAD");
									setError("");
								}}
							>
								<SelectTrigger id="issue-branch-repository">
									<SelectValue placeholder={t("linear.branchChooseRepository")} />
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
								<p className="text-sm text-muted-foreground">{t("linear.branchNoRepository")}</p>
							)}
						</div>
						<div className="space-y-2">
							<Label htmlFor="issue-branch-name">{t("git.branch")}</Label>
							<Input
								id="issue-branch-name"
								className="font-mono"
								value={branch}
								onChange={(event) => setBranch(event.target.value)}
								disabled={busy}
								required
							/>
							{exists && (
								<p className="text-sm text-error-foreground">{t("linear.branchExists")}</p>
							)}
						</div>
						<div className="space-y-2">
							<Label htmlFor="issue-branch-start">{t("git.startPoint")}</Label>
							<Combobox
								items={startPoints}
								defaultValue="HEAD"
								inputValue={startPoint}
								onInputValueChange={(value, details) => {
									if (details.reason === "input-change") setStartPoint(value);
								}}
								onValueChange={(value) => {
									if (value !== null) setStartPoint(value);
								}}
								disabled={busy || !repository.data}
							>
								<ComboboxInput
									id="issue-branch-start"
									aria-label={t("git.startPoint")}
									disabled={busy || !repository.data}
								/>
								<ComboboxContent container={contentRef.current}>
									<ComboboxList>
										{(name) => (
											<ComboboxItem key={name} value={name}>
												{name}
											</ComboboxItem>
										)}
									</ComboboxList>
								</ComboboxContent>
							</Combobox>
							<p className="text-xs text-muted-foreground">{t("linear.branchStartHint")}</p>
						</div>
						<div className="flex items-center gap-2">
							<Checkbox
								id="issue-branch-checkout"
								checked={checkout}
								onCheckedChange={(value) => setCheckout(value === true)}
								disabled={busy}
							/>
							<Label htmlFor="issue-branch-checkout" className="font-normal">
								{t("linear.branchCheckout")}
							</Label>
						</div>
						{(error || registry.error || repository.error) && (
							<ErrorNotice>{error || errorText(registry.error || repository.error)}</ErrorNotice>
						)}
						<DialogFooter>
							<Button type="button" variant="outline" disabled={busy} onClick={onClose}>
								{t("git.cancel")}
							</Button>
							<Button
								type="submit"
								disabled={busy || !selected || !repository.data || !branch.trim() || exists}
							>
								<GitBranchPlus />
								{busy ? t("git.working") : t("linear.createBranch")}
							</Button>
						</DialogFooter>
					</form>
				)}
			</DialogContent>
		</Dialog>
	);
}
