import { useState, type RefObject } from "react";
import { useTranslation } from "react-i18next";
import type { AddWorktreeInput, GitRepository } from "@/api/git";
import { Button } from "@/components/ui/button";
import {
	Combobox,
	ComboboxContent,
	ComboboxInput,
	ComboboxItem,
	ComboboxList,
} from "@/components/ui/combobox";
import { DialogFooter } from "@/components/ui/dialog";
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

export function WorktreeForm({
	repository,
	container,
	defaultBranch = "",
	busy,
	error,
	onClose,
	onCreate,
}: {
	repository: GitRepository;
	container: RefObject<HTMLDivElement | null>;
	defaultBranch?: string;
	busy: boolean;
	error: string;
	onClose(): void;
	onCreate(input: AddWorktreeInput): Promise<void>;
}) {
	const { t } = useTranslation();
	const [mode, setMode] = useState(
		repository.branches.includes(defaultBranch) ? "existing" : "new",
	);
	const [branch, setBranch] = useState(defaultBranch);
	const [startPoint, setStartPoint] = useState("HEAD");
	const [path, setPath] = useState("");
	const branchName = mode === "detached" ? "" : branch.trim();
	const exists = repository.branches.includes(branchName);
	const duplicate = mode === "new" && exists;
	const validBranch =
		mode === "detached" || (mode === "existing" ? exists : !!branchName && !duplicate);
	const name = (branchName || "worktree").replace(/[<>:"/\\|?*]/g, "-");
	const destination = path || `${repository.defaultParent}/${name}`;
	return (
		<form
			className="space-y-4"
			onSubmit={(event) => {
				event.preventDefault();
				if (busy || !validBranch) return;
				void onCreate({
					path: destination,
					branch: branchName,
					startPoint: startPoint.trim() || "HEAD",
					newBranch: mode === "new",
					detached: mode === "detached",
				});
			}}
		>
			<div className="space-y-2">
				<Label htmlFor="worktree-mode">{t("git.mode")}</Label>
				<Select
					value={mode}
					onValueChange={(value) => {
						setMode(value);
						setBranch(defaultBranch);
					}}
					disabled={busy}
				>
					<SelectTrigger id="worktree-mode">
						<SelectValue />
					</SelectTrigger>
					<SelectContent>
						<SelectItem value="new">{t("git.newBranch")}</SelectItem>
						<SelectItem value="existing">{t("git.existingBranch")}</SelectItem>
						<SelectItem value="detached">{t("git.detached")}</SelectItem>
					</SelectContent>
				</Select>
			</div>
			{mode !== "detached" && (
				<div className="space-y-2">
					<Label htmlFor="worktree-branch">{t("git.branch")}</Label>
					{mode === "existing" ? (
						<Combobox
							items={repository.branches}
							value={repository.branches.includes(branch) ? branch : null}
							inputValue={branch}
							onInputValueChange={(value, details) => {
								if (details.reason === "input-change" || details.reason === "input-clear")
									setBranch(value);
							}}
							onValueChange={(value) => {
								if (value !== null) setBranch(value);
							}}
							disabled={busy}
						>
							<ComboboxInput
								id="worktree-branch"
								aria-label={t("git.branch")}
								required
								disabled={busy}
							/>
							<ComboboxContent container={container.current}>
								<ComboboxList>
									{(name) => (
										<ComboboxItem key={name} value={name}>
											{name}
										</ComboboxItem>
									)}
								</ComboboxList>
							</ComboboxContent>
						</Combobox>
					) : (
						<Input
							id="worktree-branch"
							required
							aria-invalid={duplicate}
							value={branch}
							disabled={busy}
							onChange={(event) => setBranch(event.target.value)}
						/>
					)}
					{duplicate && <p className="text-sm text-error-foreground">{t("git.branchExists")}</p>}
				</div>
			)}
			{mode !== "existing" && (
				<div className="space-y-2">
					<Label htmlFor="worktree-start">{t("git.startPoint")}</Label>
					<Input
						id="worktree-start"
						value={startPoint}
						disabled={busy}
						onChange={(event) => setStartPoint(event.target.value)}
					/>
				</div>
			)}
			<div className="space-y-2">
				<Label htmlFor="worktree-path">{t("git.path")}</Label>
				<Input
					id="worktree-path"
					value={path}
					placeholder={destination}
					disabled={busy}
					onChange={(event) => setPath(event.target.value)}
				/>
				<p className="break-all text-xs text-muted-foreground">
					{t("git.destination", { path: destination })}
				</p>
			</div>
			{error && <ErrorNotice>{error}</ErrorNotice>}
			<DialogFooter>
				<Button type="button" variant="outline" disabled={busy} onClick={onClose}>
					{t("git.cancel")}
				</Button>
				<Button type="submit" disabled={busy || !validBranch}>
					{busy ? t("git.working") : t("git.create")}
				</Button>
			</DialogFooter>
		</form>
	);
}
