import { MessageSquare, RefreshCw } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import useSWR from "swr";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import { ErrorNotice, PageToolbar, StatePanel } from "@/components/ui/page-layout";
import { Skeleton } from "@/components/ui/skeleton";
import { getSession, getSessions } from "./api";
import { knowledgeErrorMessage } from "./errors";

export function Sessions({
	endpoint,
	initialSession,
}: {
	endpoint: string;
	initialSession?: string;
}) {
	const { t } = useTranslation();
	const { data, error, isLoading, mutate } = useSWR([endpoint, "sessions"], () =>
		getSessions(endpoint),
	);
	const [opened, setOpened] = useState(initialSession ?? "");
	return (
		<section className="space-y-3">
			<PageToolbar className="justify-end">
				<Button variant="outline" onClick={() => void mutate()}>
					<RefreshCw />
					{t("openviking.refresh")}
				</Button>
			</PageToolbar>
			{error && <ErrorNotice>{knowledgeErrorMessage(error, t("openviking.error"))}</ErrorNotice>}
			{isLoading && <Skeleton className="h-20" />}
			{data?.length === 0 && (
				<StatePanel framed icon={MessageSquare} title={t("openviking.noSessions")} />
			)}
			{Boolean(data?.length) && (
				<ul className="divide-y rounded-lg border bg-card">
					{data?.map((session) => (
						<li key={session.id} className="p-3">
							<button
								className="w-full space-y-1 text-left hover:text-primary"
								type="button"
								onClick={() => setOpened(session.id)}
							>
								<span className="block break-words text-sm font-medium">
									{session.title || session.id}
								</span>
								{session.title && (
									<code className="block break-all text-xs text-muted-foreground">
										{session.id}
									</code>
								)}
							</button>
						</li>
					))}
				</ul>
			)}
			{opened && (
				<SessionDialog key={opened} endpoint={endpoint} id={opened} onClose={() => setOpened("")} />
			)}
		</section>
	);
}

export function SessionDialog({
	endpoint,
	id,
	onClose,
}: {
	endpoint: string;
	id: string;
	onClose(): void;
}) {
	const { t } = useTranslation();
	const [archive, setArchive] = useState<string>();
	const { data, error, isLoading } = useSWR(
		[endpoint, "session", id, archive],
		() => getSession(endpoint, id, archive),
		{
			refreshInterval: (data) =>
				data?.status === "pending" || data?.status === "running" ? 2500 : 0,
			shouldRetryOnError: false,
		},
	);
	const current = Number(data?.archive?.split("_").at(-1));
	const navigateArchive = (number: number) =>
		setArchive("archive_" + String(number).padStart(3, "0"));
	return (
		<Dialog
			open
			onOpenChange={(open) => {
				if (!open) onClose();
			}}
		>
			<DialogContent className="ud-sm:max-w-3xl">
				<DialogHeader>
					<DialogTitle>{data?.title || t("openviking.sessions")}</DialogTitle>
					<DialogDescription className="break-all font-mono">{id}</DialogDescription>
				</DialogHeader>
				{error && <ErrorNotice>{knowledgeErrorMessage(error, t("openviking.error"))}</ErrorNotice>}
				{isLoading && <Skeleton className="h-24" />}
				{data && (
					<div className="space-y-4">
						<div className="flex flex-wrap items-center gap-2">
							{data.status && (
								<Badge variant="outline">
									{t("openviking.memoryStatus." + data.status, { defaultValue: data.status })}
								</Badge>
							)}
							{data.memoryCount !== undefined && (
								<span className="text-sm text-muted-foreground">
									{t("openviking.extractedCount", { count: data.memoryCount })}
								</span>
							)}
							{data.archives > 1 && (
								<>
									<Button
										variant="outline"
										size="sm"
										disabled={!current || current <= 1}
										onClick={() => navigateArchive(current - 1)}
									>
										{t("openviking.previousRound")}
									</Button>
									<span className="text-sm text-muted-foreground">
										{current} / {data.archives}
									</span>
									<Button
										variant="outline"
										size="sm"
										disabled={!current || current >= data.archives}
										onClick={() => navigateArchive(current + 1)}
									>
										{t("openviking.nextRound")}
									</Button>
								</>
							)}
						</div>
						{data.messages?.length ? (
							<ul className="max-h-[55vh] space-y-3 overflow-y-auto">
								{data.messages.map(({ role, text }, index) => (
									<li key={index} className="space-y-1 rounded-lg border bg-card p-3">
										<span className="text-xs font-medium text-muted-foreground">
											{t("openviking.message." + role, { defaultValue: role })}
										</span>
										<p className="text-sm break-words whitespace-pre-wrap">{text}</p>
									</li>
								))}
							</ul>
						) : (
							<p className="text-sm text-muted-foreground">{t("openviking.noMessages")}</p>
						)}
					</div>
				)}
			</DialogContent>
		</Dialog>
	);
}
