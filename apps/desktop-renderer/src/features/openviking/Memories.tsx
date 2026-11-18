import { Plus, SearchX } from "lucide-react";
import { useState } from "react";
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
import { Label } from "@/components/ui/label";
import { ErrorNotice, PageToolbar, SearchInput, StatePanel } from "@/components/ui/page-layout";
import { Skeleton } from "@/components/ui/skeleton";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { getWorkspaceAgent, listMemories, readMemory, setWorkspaceAgent, writeMemory } from "./api";
import { knowledgeErrorMessage } from "./errors";
import { SessionDialog } from "./Sessions";

export function Memories({ endpoint }: { endpoint: string }) {
	const { t } = useTranslation();
	const agent = useSWR([endpoint, "agent"], () => getWorkspaceAgent(endpoint));
	const [draft, setDraft] = useState("");
	const [query, setQuery] = useState("");
	const hits = useSWR([endpoint, "memories", query], () => listMemories(endpoint, query));
	const [uri, setURI] = useState("");
	const content = useSWR(uri ? [endpoint, "memory", uri] : null, () => readMemory(endpoint, uri));
	const [text, setText] = useState("");
	const [composing, setComposing] = useState(false);
	const [receipt, setReceipt] = useState("");
	const [openReceipt, setOpenReceipt] = useState(false);
	const [pending, setPending] = useState(false);
	const [error, setError] = useState("");

	async function change(action: () => Promise<void>) {
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
		<section className="space-y-4">
			<form
				onSubmit={(event) => {
					event.preventDefault();
					setQuery(draft.trim());
					if (draft.trim() === query) void hits.mutate();
				}}
			>
				<PageToolbar>
					<SearchInput
						groupClassName="min-w-40 max-w-sm flex-1"
						aria-label={t("openviking.searchMemories")}
						placeholder={t("openviking.searchMemories")}
						value={draft}
						onChange={(event) => setDraft(event.target.value)}
					/>
					<Button type="submit" variant="outline">
						{t("openviking.search")}
					</Button>
					<Button
						type="button"
						className="ms-auto"
						aria-expanded={composing}
						aria-controls="openviking-memory-editor"
						onClick={() => setComposing((open) => !open)}
						disabled={pending}
					>
						<Plus />
						{t("openviking.newMemory")}
					</Button>
				</PageToolbar>
			</form>
			<div className="flex items-center justify-between gap-4 rounded-lg border bg-card p-4">
				<div className="space-y-1">
					<Label htmlFor="openviking-auto-memory">{t("openviking.automaticMemory")}</Label>
					<p className="text-sm text-muted-foreground">{t("openviking.automaticMemoryHint")}</p>
				</div>
				<Switch
					id="openviking-auto-memory"
					checked={agent.data?.automatic ?? false}
					disabled={!agent.data || pending}
					onCheckedChange={(automatic) =>
						void change(async () => {
							const next = await setWorkspaceAgent(endpoint, { automatic });
							await agent.mutate(next, { revalidate: false });
						})
					}
				/>
			</div>
			{(error || agent.error || hits.error) && (
				<ErrorNotice>
					{error || knowledgeErrorMessage(agent.error || hits.error, t("openviking.error"))}
				</ErrorNotice>
			)}
			<form
				id="openviking-memory-editor"
				className="space-y-2 rounded-lg border border-border bg-card p-4"
				hidden={!composing}
				onSubmit={(event) => {
					event.preventDefault();
					void change(async () => {
						const session = await writeMemory(endpoint, text);
						setText("");
						setComposing(false);
						setReceipt(session.id);
					});
				}}
			>
				<Label htmlFor="openviking-memory-text">{t("openviking.newMemory")}</Label>
				<Textarea
					id="openviking-memory-text"
					value={text}
					onChange={(event) => setText(event.target.value)}
					disabled={pending}
					placeholder={t("openviking.memoryPlaceholder")}
					maxLength={32 * 1024}
					rows={3}
				/>
				<Button type="submit" disabled={pending || !text.trim()}>
					{t("openviking.writeMemory")}
				</Button>
			</form>
			{receipt && (
				<p
					role="status"
					className="flex flex-wrap items-center gap-2 text-sm text-muted-foreground"
				>
					{t("openviking.memoryQueued")}
					<Button type="button" variant="link" size="sm" onClick={() => setOpenReceipt(true)}>
						{t("openviking.viewSession")}
					</Button>
				</p>
			)}
			{hits.isLoading && <Skeleton className="h-20" />}
			{hits.data?.length === 0 && (
				<StatePanel framed icon={SearchX} title={t("openviking.noMemories")} />
			)}
			{Boolean(hits.data?.length) && (
				<ul className="divide-y rounded-lg border bg-card">
					{hits.data?.map((match) => (
						<li key={match.uri} className="space-y-1 p-3">
							<button
								className="break-all text-left font-mono text-sm text-primary hover:underline"
								type="button"
								onClick={() => setURI(match.uri)}
							>
								{match.uri.split("/").at(-1)}
							</button>
							{match.abstract && <p className="text-sm text-muted-foreground">{match.abstract}</p>}
						</li>
					))}
				</ul>
			)}
			{uri && (
				<Dialog
					open
					onOpenChange={(open) => {
						if (!open) setURI("");
					}}
				>
					<DialogContent className="ud-sm:max-w-3xl">
						<DialogHeader>
							<DialogTitle>{t("openviking.memories")}</DialogTitle>
							<DialogDescription className="break-all">{uri}</DialogDescription>
						</DialogHeader>
						{content.isLoading && <Skeleton className="h-20" />}
						{content.error && (
							<ErrorNotice>
								{knowledgeErrorMessage(content.error, t("openviking.error"))}
							</ErrorNotice>
						)}
						{content.data && (
							<pre className="max-h-[55vh] overflow-y-auto text-sm break-words whitespace-pre-wrap">
								{content.data.text}
							</pre>
						)}
					</DialogContent>
				</Dialog>
			)}
			{openReceipt && receipt && (
				<SessionDialog endpoint={endpoint} id={receipt} onClose={() => setOpenReceipt(false)} />
			)}
		</section>
	);
}
