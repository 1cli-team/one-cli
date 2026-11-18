import { BookOpen, SearchX } from "lucide-react";
import { Tabs } from "radix-ui";
import { useState } from "react";
import { useSearchParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import useSWR from "swr";
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
import {
	ErrorNotice,
	PageContent,
	PageHeader,
	PageTab,
	PageTabList,
	SearchInput,
	StatePanel,
} from "@/components/ui/page-layout";
import { Skeleton } from "@/components/ui/skeleton";
import { EnvironmentLink } from "@/features/environment-context/EnvironmentLink";
import { knowledgeErrorMessage } from "./errors";
import { ImportDocument } from "./ImportDocument";
import { Memories } from "./Memories";
import { Sessions } from "./Sessions";
import { Skills } from "./Skills";
import {
	bindKnowledge,
	browseKnowledge,
	findKnowledge,
	getKnowledge,
	knowledgeKey,
	readKnowledge,
	unbindKnowledge,
	type KnowledgeBinding,
	type KnowledgeContent,
	type KnowledgeDirectories,
	type KnowledgeMatch,
} from "./api";

export function KnowledgePage({ entryId }: { entryId: string }) {
	const { t } = useTranslation();
	const endpoint = knowledgeKey(entryId);
	const { data, error, mutate } = useSWR(endpoint, getKnowledge, { shouldRetryOnError: false });
	const [editing, setEditing] = useState(false);
	const [searchParams, setSearchParams] = useSearchParams();
	const session = searchParams.get("session") ?? undefined;
	const requested = searchParams.get("section");
	const section =
		requested === "memories" || requested === "sessions" || requested === "skills"
			? requested
			: "resources";
	function selectSection(value: string) {
		const next = new URLSearchParams(searchParams);
		next.delete("session");
		if (value === "resources") next.delete("section");
		else next.set("section", value);
		setSearchParams(next, { replace: true });
	}
	const binding = data?.binding;

	return (
		<PageContent scroll>
			<PageHeader
				title={t("openviking.title")}
				description={
					data?.configured && !data.needsRebind && binding && !editing ? (
						<code className="font-mono text-xs wrap-anywhere">{binding.uri}</code>
					) : undefined
				}
				actions={
					data?.configured &&
					!data.needsRebind &&
					binding &&
					!editing && (
						<Button variant="outline" onClick={() => setEditing(true)}>
							{t("openviking.configure")}
						</Button>
					)
				}
			/>
			{error && (
				<ErrorNotice
					action={
						<Button variant="outline" onClick={() => void mutate()}>
							{t("workspaces.retry")}
						</Button>
					}
				>
					{knowledgeErrorMessage(error, t("openviking.error"))}
				</ErrorNotice>
			)}
			{!data && !error && <Skeleton className="h-44 w-full" />}
			{data && !data.configured && (
				<StatePanel
					framed
					icon={BookOpen}
					title={t("openviking.connection")}
					description={t("openviking.configureFirst")}
				>
					<Button asChild variant="outline">
						<EnvironmentLink to="/settings/openviking">{t("openviking.settings")}</EnvironmentLink>
					</Button>
				</StatePanel>
			)}
			{data?.configured && data.needsRebind && (
				<ErrorNotice>{t("openviking.rebindRequired")}</ErrorNotice>
			)}
			{data?.configured &&
				(!binding || editing || data.needsRebind ? (
					<BindingForm
						endpoint={endpoint}
						binding={binding ?? undefined}
						onClose={editing && !data.needsRebind ? () => setEditing(false) : undefined}
						onSaved={(result) => {
							void mutate(result, { revalidate: false });
							setEditing(false);
						}}
					/>
				) : (
					<Tabs.Root
						value={section}
						onValueChange={selectSection}
						className="flex min-w-0 flex-col gap-4"
					>
						<PageTabList aria-label={t("openviking.title")}>
							{(["resources", "memories", "sessions", "skills"] as const).map((item) => (
								<PageTab key={item} value={item}>
									{t("openviking." + item)}
								</PageTab>
							))}
						</PageTabList>
						<Tabs.Content value="resources" className="min-w-0">
							<KnowledgeSearch key={binding.uri} endpoint={endpoint} />
						</Tabs.Content>
						<Tabs.Content value="memories" className="min-w-0">
							<Memories endpoint={endpoint} />
						</Tabs.Content>
						<Tabs.Content value="sessions" className="min-w-0">
							<Sessions key={session ?? "list"} endpoint={endpoint} initialSession={session} />
						</Tabs.Content>
						<Tabs.Content value="skills" className="min-w-0">
							<Skills endpoint={endpoint} />
						</Tabs.Content>
					</Tabs.Root>
				))}
		</PageContent>
	);
}

function BindingForm({
	endpoint,
	binding,
	onClose,
	onSaved,
}: {
	endpoint: string;
	binding?: KnowledgeBinding;
	onClose?(): void;
	onSaved(result: Awaited<ReturnType<typeof getKnowledge>>): void;
}) {
	const { t } = useTranslation();
	const [uri, setURI] = useState(binding?.uri ?? "");
	const [busy, setBusy] = useState(false);
	const [browsing, setBrowsing] = useState(false);
	const [directories, setDirectories] = useState<KnowledgeDirectories | null>(null);
	const disabled = busy || browsing;
	const [error, setError] = useState("");
	async function update(remove: boolean) {
		setBusy(true);
		setError("");
		try {
			const result = remove
				? await unbindKnowledge(endpoint)
				: await bindKnowledge(endpoint, { uri });
			onSaved(result);
		} catch (err) {
			setError(knowledgeErrorMessage(err, t("openviking.error")));
		} finally {
			setBusy(false);
		}
	}
	async function browse(directory: string) {
		setBrowsing(true);
		setError("");
		try {
			setDirectories(await browseKnowledge(endpoint, { uri: directory }));
		} catch (err) {
			setDirectories(null);
			setError(knowledgeErrorMessage(err, t("openviking.error")));
		} finally {
			setBrowsing(false);
		}
	}
	return (
		<form
			className="flex max-w-2xl flex-col gap-4 rounded-lg border bg-card p-4"
			onSubmit={(event) => {
				event.preventDefault();
				void update(false);
			}}
		>
			<p className="text-sm text-muted-foreground">{t("openviking.bindingHint")}</p>
			<Label className="flex flex-col items-start gap-2">
				{t("openviking.uri")}
				<Input
					required
					value={uri}
					placeholder="viking://resources/workspace"
					onChange={(e) => {
						setURI(e.target.value);
						setDirectories(null);
					}}
					disabled={disabled}
				/>
			</Label>
			<div>
				<Button type="button" variant="outline" onClick={() => void browse("")} disabled={disabled}>
					{t(browsing ? "openviking.loadingDirectories" : "openviking.browseDirectories")}
				</Button>
				{directories && (
					<div className="mt-2 space-y-2 rounded-md border p-3">
						<p className="break-all text-sm font-mono text-muted-foreground">
							{directories.current}
						</p>
						{directories.parent && (
							<Button
								type="button"
								variant="ghost"
								size="sm"
								disabled={disabled}
								onClick={() => void browse(directories.parent ?? "")}
							>
								{t("openviking.parentDirectory")}
							</Button>
						)}
						<ul className="max-h-48 overflow-y-auto">
							{directories.entries.map((directory) => (
								<li key={directory.uri}>
									<Button
										type="button"
										variant="ghost"
										size="sm"
										className="w-full justify-start"
										disabled={disabled}
										onClick={() => void browse(directory.uri)}
									>
										<span className="truncate">{directory.name || directory.uri}</span>
									</Button>
								</li>
							))}
						</ul>
						<Button
							type="button"
							size="sm"
							disabled={disabled}
							onClick={() => {
								setURI(directories.current);
								setDirectories(null);
							}}
						>
							{t("openviking.selectDirectory")}
						</Button>
					</div>
				)}
			</div>
			{error && <ErrorNotice>{error}</ErrorNotice>}
			<div className="flex flex-wrap gap-2">
				<Button type="submit" disabled={disabled}>
					{t(busy ? "openviking.saving" : "openviking.save")}
				</Button>
				{onClose && (
					<Button type="button" variant="outline" onClick={onClose} disabled={disabled}>
						{t("form.cancel")}
					</Button>
				)}
				{binding && (
					<Button
						type="button"
						variant="ghost"
						onClick={() => void update(true)}
						disabled={disabled}
					>
						{t("openviking.unbind")}
					</Button>
				)}
			</div>
		</form>
	);
}

function KnowledgeSearch({ endpoint }: { endpoint: string }) {
	const { t } = useTranslation();
	const [query, setQuery] = useState("");
	const [matches, setMatches] = useState<KnowledgeMatch[] | null>(null);
	const [content, setContent] = useState<KnowledgeContent | null>(null);
	const [error, setError] = useState("");
	const [busy, setBusy] = useState<"find" | "read" | null>(null);

	async function search(value: string) {
		if (!value.trim() || busy) return;
		setQuery(value);
		setBusy("find");
		setMatches(null);
		setContent(null);
		setError("");
		try {
			setMatches(await findKnowledge(endpoint, value.trim()));
		} catch (err) {
			setError(knowledgeErrorMessage(err, t("openviking.error")));
		} finally {
			setBusy(null);
		}
	}

	async function open(uri: string) {
		setBusy("read");
		setContent(null);
		setError("");
		try {
			setContent(await readKnowledge(endpoint, uri));
		} catch (err) {
			setError(knowledgeErrorMessage(err, t("openviking.error")));
		} finally {
			setBusy(null);
		}
	}

	return (
		<div className="flex min-w-0 flex-col gap-4">
			<ImportDocument
				endpoint={endpoint}
				onSearch={(name) => void search(name)}
				searchBusy={busy !== null}
				searchControl={
					<form
						className="flex min-w-0 flex-1 basis-60 items-center gap-2"
						onSubmit={(event) => {
							event.preventDefault();
							void search(query);
						}}
					>
						<SearchInput
							aria-label={t("openviking.query")}
							placeholder={t("openviking.query")}
							value={query}
							onChange={(e) => setQuery(e.target.value)}
						/>
						<Button type="submit" disabled={!query.trim() || busy !== null}>
							{t("openviking.search")}
						</Button>
					</form>
				}
			/>
			{busy && (
				<p role="status" className="text-sm text-muted-foreground">
					{t(busy === "find" ? "openviking.searching" : "openviking.reading")}
				</p>
			)}
			{error && <ErrorNotice>{error}</ErrorNotice>}
			{matches?.length === 0 && (
				<StatePanel framed icon={SearchX} title={t("openviking.noResults")} />
			)}
			{matches && matches.length > 0 && (
				<ul aria-label={t("openviking.results")} className="divide-y rounded-lg border bg-card">
					{matches.map((match, index) => (
						<li key={match.uri + "\0" + index} className="flex flex-col items-start gap-1 p-4">
							<button
								type="button"
								className="text-left font-mono text-sm break-all text-primary hover:underline disabled:opacity-60"
								onClick={() => void open(match.uri)}
								disabled={busy !== null}
							>
								{match.uri}
							</button>
							{match.abstract && (
								<p className="text-sm break-words text-muted-foreground">{match.abstract}</p>
							)}
						</li>
					))}
				</ul>
			)}
			<Dialog
				open={content !== null}
				onOpenChange={(open) => {
					if (!open) setContent(null);
				}}
			>
				{content && (
					<DialogContent className="ud-sm:max-w-3xl">
						<DialogHeader>
							<DialogTitle>
								{t(content.isDirectory ? "openviking.overview" : "openviking.source")}
							</DialogTitle>
							<DialogDescription className="break-all font-mono">{content.uri}</DialogDescription>
						</DialogHeader>
						<pre
							tabIndex={0}
							className="max-h-[55vh] min-w-0 overflow-y-auto text-sm break-words whitespace-pre-wrap"
						>
							{content.text}
						</pre>
					</DialogContent>
				)}
			</Dialog>
		</div>
	);
}
