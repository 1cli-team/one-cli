import { Plus, SearchX } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import useSWR from "swr";
import {
	AlertDialog,
	AlertDialogCancel,
	AlertDialogContent,
	AlertDialogDescription,
	AlertDialogFooter,
	AlertDialogHeader,
	AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ErrorNotice, PageToolbar, SearchInput, StatePanel } from "@/components/ui/page-layout";
import { Skeleton } from "@/components/ui/skeleton";
import { Textarea } from "@/components/ui/textarea";
import {
	deleteSkill,
	getSkill,
	getSkills,
	getWorkspaceAgent,
	saveSkill,
	setWorkspaceAgent,
	type SkillRef,
	type SkillScope,
} from "./api";
import { knowledgeErrorMessage } from "./errors";

export function Skills({ endpoint }: { endpoint: string }) {
	const { t } = useTranslation();
	const [draft, setDraft] = useState("");
	const [query, setQuery] = useState("");
	const skills = useSWR([endpoint, "skills", query], () => getSkills(endpoint, query));
	const agent = useSWR([endpoint, "agent"], () => getWorkspaceAgent(endpoint));
	const [selected, setSelected] = useState<SkillRef>();
	const [deleting, setDeleting] = useState<SkillRef>();
	const [adding, setAdding] = useState(false);
	const [pending, setPending] = useState(false);
	const [error, setError] = useState("");
	const [feedback, setFeedback] = useState("");

	async function mutate(action: () => Promise<void>) {
		setPending(true);
		setError("");
		try {
			await action();
		} catch (err) {
			setError(knowledgeErrorMessage(err, t("openviking.error")));
		} finally {
			setPending(false);
		}
	}

	return (
		<section className="space-y-3">
			<form
				onSubmit={(event) => {
					event.preventDefault();
					setQuery(draft.trim());
					if (query === draft.trim()) void skills.mutate();
				}}
			>
				<PageToolbar>
					<SearchInput
						groupClassName="min-w-40 max-w-sm flex-1"
						aria-label={t("openviking.searchSkills")}
						placeholder={t("openviking.searchSkills")}
						value={draft}
						onChange={(event) => setDraft(event.target.value)}
					/>
					<Button type="submit" variant="outline">
						{t("openviking.search")}
					</Button>
					<Button type="button" className="ms-auto" onClick={() => setAdding(true)}>
						<Plus />
						{t("openviking.addSkill")}
					</Button>
				</PageToolbar>
			</form>
			{(error || skills.error || agent.error) && (
				<ErrorNotice>
					{error || knowledgeErrorMessage(skills.error || agent.error, t("openviking.error"))}
				</ErrorNotice>
			)}
			{feedback && (
				<p className="text-sm text-muted-foreground" role="status">
					{feedback}
				</p>
			)}
			{skills.isLoading && <Skeleton className="h-20" />}
			{skills.data?.length === 0 && (
				<StatePanel framed icon={SearchX} title={t("openviking.noSkills")} />
			)}
			{Boolean(skills.data?.length) && (
				<ul className="divide-y rounded-lg border bg-card">
					{skills.data?.map((skill) => {
						const ref = { scope: skill.scope, name: skill.name };
						const enabled =
							agent.data?.skills?.some(
								({ scope, name }) => scope === ref.scope && name === ref.name,
							) ?? false;
						return (
							<li
								key={skill.scope + "/" + skill.name}
								className="flex flex-wrap items-start justify-between gap-3 p-3"
							>
								<div className="min-w-0 flex-1 space-y-1">
									<div className="flex flex-wrap items-center gap-2">
										<button
											type="button"
											className="break-all text-left font-medium hover:text-primary"
											onClick={() => setSelected(ref)}
										>
											{skill.name}
										</button>
										<Badge variant="outline">{t("openviking.skillScope." + skill.scope)}</Badge>
									</div>
									{skill.description && (
										<p className="text-sm break-words text-muted-foreground">{skill.description}</p>
									)}
								</div>
								<div className="flex flex-wrap gap-2">
									<Button
										variant={enabled ? "secondary" : "outline"}
										size="sm"
										disabled={!agent.data || pending}
										onClick={() =>
											void mutate(async () => {
												const result = await setWorkspaceAgent(endpoint, {
													skill: ref,
													enabled: !enabled,
												});
												await agent.mutate(result, { revalidate: false });
											})
										}
									>
										{t(enabled ? "openviking.disableSkill" : "openviking.enableSkill")}
									</Button>
									<Button
										variant="ghost"
										size="sm"
										onClick={() => setDeleting(ref)}
										disabled={pending}
									>
										{t("openviking.deleteSkill")}
									</Button>
								</div>
							</li>
						);
					})}
				</ul>
			)}
			{(adding || selected) && (
				<SkillDialog
					key={selected ? selected.scope + "/" + selected.name : "new"}
					endpoint={endpoint}
					skill={selected}
					onClose={() => {
						setAdding(false);
						setSelected(undefined);
					}}
					onSaved={(taskId) => {
						setFeedback(t(taskId ? "openviking.skillQueued" : "openviking.skillSaved"));
						setAdding(false);
						setSelected(undefined);
						void skills.mutate();
					}}
				/>
			)}
			{deleting && (
				<AlertDialog
					open
					onOpenChange={(open) => {
						if (!open && !pending) setDeleting(undefined);
					}}
				>
					<AlertDialogContent>
						<AlertDialogHeader>
							<AlertDialogTitle>{t("openviking.deleteSkill")}</AlertDialogTitle>
							<AlertDialogDescription>
								{t("openviking.deleteSkillHint", { name: deleting.name })}
							</AlertDialogDescription>
						</AlertDialogHeader>
						{error && <ErrorNotice>{error}</ErrorNotice>}
						<AlertDialogFooter>
							<AlertDialogCancel asChild>
								<Button variant="outline" disabled={pending}>
									{t("form.cancel")}
								</Button>
							</AlertDialogCancel>
							<Button
								variant="destructive"
								disabled={pending}
								onClick={() =>
									void mutate(async () => {
										await deleteSkill(endpoint, deleting);
										setDeleting(undefined);
										await skills.mutate();
									})
								}
							>
								{t("openviking.deleteSkill")}
							</Button>
						</AlertDialogFooter>
					</AlertDialogContent>
				</AlertDialog>
			)}
		</section>
	);
}

function SkillDialog({
	endpoint,
	skill,
	onClose,
	onSaved,
}: {
	endpoint: string;
	skill?: SkillRef;
	onClose(): void;
	onSaved(taskId: string): void;
}) {
	const { t } = useTranslation();
	const loaded = useSWR(skill ? [endpoint, "skill", skill.scope, skill.name] : null, () =>
		getSkill(endpoint, skill!),
	);
	const [scope, setScope] = useState<SkillScope>(skill?.scope ?? "user");
	const [draft, setDraft] = useState<string>();
	const [error, setError] = useState("");
	const [pending, setPending] = useState(false);
	const attachments = loaded.data?.attachments;
	const source = draft ?? (attachments ? "" : (loaded.data?.content ?? ""));
	async function save() {
		setPending(true);
		setError("");
		try {
			const result = await saveSkill(endpoint, {
				scope,
				name: skill?.name,
				source,
				replace: Boolean(skill),
			});
			onSaved(result.taskId);
		} catch (err) {
			setError(knowledgeErrorMessage(err, t("openviking.error")));
		} finally {
			setPending(false);
		}
	}
	return (
		<Dialog
			open
			onOpenChange={(open) => {
				if (!open && !pending) onClose();
			}}
		>
			<DialogContent className="ud-sm:max-w-3xl">
				<DialogHeader>
					<DialogTitle>{skill?.name ?? t("openviking.addSkill")}</DialogTitle>
					<DialogDescription>
						{t(attachments ? "openviking.skillPackageHint" : "openviking.skillSourceHint")}
					</DialogDescription>
				</DialogHeader>
				{(error || loaded.error) && (
					<ErrorNotice>
						{error || knowledgeErrorMessage(loaded.error, t("openviking.error"))}
					</ErrorNotice>
				)}
				{loaded.isLoading && <Skeleton className="h-32" />}
				{(!skill || loaded.data) && (
					<form
						className="space-y-3"
						onSubmit={(event) => {
							event.preventDefault();
							void save();
						}}
					>
						{!skill && (
							<div className="space-y-2">
								<Label htmlFor="openviking-skill-scope">{t("openviking.skillScopeLabel")}</Label>
								<select
									className="rounded-md border bg-background px-2 py-1 text-sm"
									id="openviking-skill-scope"
									value={scope}
									onChange={(event) =>
										setScope(event.target.value === "shared" ? "shared" : "user")
									}
								>
									<option value="user">{t("openviking.skillScope.user")}</option>
									<option value="shared">{t("openviking.skillScope.shared")}</option>
								</select>
							</div>
						)}
						{attachments && loaded.data?.content && (
							<pre className="max-h-48 overflow-y-auto rounded-md border p-2 text-sm break-words whitespace-pre-wrap">
								{loaded.data.content}
							</pre>
						)}
						{!attachments && (
							<div className="space-y-2">
								<Label htmlFor="openviking-skill-file">{t("openviking.skillFile")}</Label>
								<Input
									id="openviking-skill-file"
									type="file"
									accept=".md,text/markdown"
									disabled={pending}
									onChange={(event) => {
										const file = event.target.files?.[0];
										if (!file) return;
										setError("");
										if (file.size > 512 * 1024) {
											setError(t("openviking.skillSourceHint"));
											return;
										}
										void file
											.text()
											.then(setDraft)
											.catch((err) => setError(knowledgeErrorMessage(err, t("openviking.error"))));
									}}
								/>
							</div>
						)}
						<div className="space-y-2">
							<Label htmlFor="openviking-skill-source">{t("openviking.skillSource")}</Label>
							<Textarea
								id="openviking-skill-source"
								rows={attachments ? 2 : 8}
								value={source}
								disabled={pending}
								className="font-mono"
								onChange={(event) => setDraft(event.target.value)}
							/>
						</div>
						<div className="flex justify-end gap-2">
							<Button type="button" variant="outline" onClick={onClose} disabled={pending}>
								{t("form.cancel")}
							</Button>
							<Button
								type="submit"
								disabled={pending || !source.trim() || Boolean(skill && draft === undefined)}
							>
								{t(pending ? "openviking.saving" : "openviking.saveSkill")}
							</Button>
						</div>
					</form>
				)}
			</DialogContent>
		</Dialog>
	);
}
