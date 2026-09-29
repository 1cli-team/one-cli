import { useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import {
	Copy,
	ExternalLink,
	LoaderCircle,
	Pause,
	Play,
	RotateCw,
	Search,
	Square,
	Terminal,
} from "lucide-react";
import { toast } from "sonner";
import { controlService, watchService } from "@/api/services";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { InputGroup, InputGroupAddon, InputGroupInput } from "@/components/ui/input-group";
import {
	manifestDraftKey,
	useManifestDraftStore,
} from "@/features/manifest-draft/manifest-draft-store";
import { consoleText, renderConsole } from "@/features/services/console";
import type { DevService, ProjectSettings } from "@/types/api";

export function mergeService(previous: DevService | undefined, next: DevService): DevService {
	const retained =
		previous?.id === next.id ? previous.logs.filter((line) => line.seq >= next.firstSeq) : [];
	const last = retained.at(-1)?.seq ?? 0;
	return {
		...next,
		logs: [...retained, ...next.logs.filter((line) => line.seq > last)].slice(-2048),
	};
}
export function ServicePanel({
	project,
	environment,
	entryId,
	readOnly,
}: {
	project: ProjectSettings;
	environment: string;
	entryId?: string;
	readOnly?: boolean;
}) {
	const { t } = useTranslation();
	const [state, setState] = useState<DevService>();
	const [connected, setConnected] = useState(false);
	const [pending, setPending] = useState(false);
	const [error, setError] = useState("");
	const [query, setQuery] = useState("");
	const [follow, setFollow] = useState(true);
	const viewport = useRef<HTMLPreElement>(null);
	const draft = useManifestDraftStore((store) => store.drafts[manifestDraftKey(entryId)]);
	useEffect(
		() =>
			watchService(
				project.name,
				entryId,
				(next) => setState((previous) => mergeService(previous, next)),
				setConnected,
			),
		[entryId, project.name],
	);
	const active = state && ["preparing", "running", "stopping"].includes(state.status);
	const busy = pending || state?.status === "stopping";
	const hasDev = project.devAvailable === true;
	const raw = state?.logs.map((line) => line.text).join("") ?? "";
	const text = useMemo(() => consoleText(raw), [raw]);
	const visible = useMemo(
		() =>
			query
				? text
						.split("\n")
						.filter((line) => line.toLocaleLowerCase().includes(query.toLocaleLowerCase()))
						.join("\n")
				: raw,
		[query, text, raw],
	);
	const consoleContent = useMemo(() => renderConsole(visible), [visible]);
	useEffect(() => {
		if (follow && viewport.current) viewport.current.scrollTop = viewport.current.scrollHeight;
	}, [visible, follow]);
	async function control(action: "start" | "stop" | "restart") {
		setPending(true);
		setError("");
		try {
			await controlService(project.name, entryId, action, environment);
		} catch (failure) {
			setError((failure as { message?: string }).message ?? t("services.actionFailed"));
		} finally {
			setPending(false);
		}
	}
	async function copy() {
		try {
			await navigator.clipboard.writeText(query ? visible : text);
			toast.success(t("services.copied"));
		} catch {
			toast.error(t("services.copyFailed"));
		}
	}
	const status = state?.status ?? "stopped";
	return (
		<div className="space-y-4">
			<section
				aria-label={t("services.title")}
				className="space-y-4 rounded-lg border border-border bg-card p-4 ud-sm:p-5"
			>
				<div className="flex flex-wrap items-start justify-between gap-4">
					<div className="min-w-0 space-y-2">
						<div className="flex flex-wrap items-center gap-2">
							<h3 className="text-base font-semibold">{t("services.title")}</h3>
							<Badge
								variant={
									status === "running"
										? "success"
										: status === "failed"
											? "error"
											: active
												? "warning"
												: "muted"
								}
							>
								{status === "preparing" || status === "stopping" ? (
									<LoaderCircle className="animate-spin" />
								) : null}
								{t(`services.status.${status}`)}
							</Badge>
						</div>
						<p className="text-sm text-muted-foreground">{t("services.description")}</p>
						<p className="font-mono text-xs text-muted-foreground break-all">
							one dev -p {project.name} --env {active ? state.environment : environment}
						</p>
					</div>
					<div className="flex flex-wrap gap-2">
						{active ? (
							<>
								<Button
									variant="outline"
									disabled={busy || !connected || readOnly || !!draft}
									onClick={() => void control("restart")}
								>
									<RotateCw />
									{t("services.restart")}
								</Button>
								<Button
									variant="outline"
									disabled={busy || !connected}
									onClick={() => void control("stop")}
								>
									<Square />
									{t("services.stop")}
								</Button>
							</>
						) : (
							<Button
								disabled={pending || !connected || readOnly || !!draft || !hasDev}
								onClick={() => void control("start")}
							>
								{pending ? <LoaderCircle className="animate-spin" /> : <Play />}
								{t("services.start")}
							</Button>
						)}
					</div>
				</div>
				{!!draft && <p className="text-sm text-warning-foreground">{t("services.saveFirst")}</p>}
				{!hasDev && <p className="text-sm text-muted-foreground">{t("services.noDev")}</p>}
				{active && state.environment !== environment && (
					<p className="text-sm text-warning-foreground">
						{t("services.environmentMismatch", {
							environment: state.environment,
							next: environment,
						})}
					</p>
				)}
				{state?.exitCode !== undefined && !active && (
					<p className="text-xs text-muted-foreground">
						{t("services.exitCode", { code: state.exitCode })}
					</p>
				)}
				{error && (
					<Alert variant="destructive">
						<AlertDescription>{error}</AlertDescription>
					</Alert>
				)}
				<div className="space-y-2 border-t border-border pt-3">
					{state?.endpoints.length ? (
						state.endpoints.map((endpoint) => (
							<div key={endpoint.url} className="flex flex-wrap items-center justify-between gap-2">
								<div className="min-w-0">
									<p className="font-mono text-sm break-all">{endpoint.url}</p>
									<p className="text-xs text-muted-foreground">
										{t(
											endpoint.reachable && active ? "services.reachable" : "services.unreachable",
										)}
									</p>
								</div>
								{endpoint.reachable && active ? (
									<Button variant="outline" asChild>
										<a href={endpoint.url} target="_blank" rel="noreferrer noopener">
											<ExternalLink />
											{t("services.open")}
										</a>
									</Button>
								) : (
									<Button variant="outline" disabled>
										<ExternalLink />
										{t("services.open")}
									</Button>
								)}
							</div>
						))
					) : (
						<p className="text-sm text-muted-foreground">
							{t(active ? "services.waitingURL" : "services.urlHint")}
						</p>
					)}
				</div>
			</section>
			<section
				className="overflow-hidden rounded-lg border border-border bg-card"
				aria-label={t("services.console")}
			>
				<div className="flex flex-wrap items-center justify-between gap-3 border-b border-border p-3">
					<div className="flex items-center gap-2">
						<Terminal className="size-4 text-muted-foreground" />
						<h3 className="text-sm font-semibold">{t("services.console")}</h3>
						<span role="status" className="text-xs text-muted-foreground">
							{t(connected ? "services.live" : "services.reconnecting")}
						</span>
					</div>
					<div className="flex flex-wrap items-center gap-2">
						<InputGroup className="w-44">
							<InputGroupAddon>
								<Search />
							</InputGroupAddon>
							<InputGroupInput
								value={query}
								onChange={(event) => setQuery(event.target.value)}
								placeholder={t("services.search")}
								aria-label={t("services.search")}
							/>
						</InputGroup>
						<Button
							variant="ghost"
							size="sm"
							aria-pressed={!follow}
							onClick={() => setFollow(!follow)}
						>
							{follow ? <Pause /> : <Play />}
							{t(follow ? "services.pause" : "services.follow")}
						</Button>
						<Button variant="ghost" size="sm" disabled={!raw} onClick={() => void copy()}>
							<Copy />
							{t("services.copy")}
						</Button>
					</div>
				</div>
				<pre
					ref={viewport}
					tabIndex={0}
					aria-label={t("services.console")}
					className="h-80 overflow-auto bg-muted/30 p-4 font-mono text-xs leading-6 whitespace-pre-wrap break-all outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring ud-md:h-96"
				>
					{raw
						? visible
							? consoleContent
							: t("services.noMatches")
						: t(active ? "services.waitingLogs" : "services.emptyLogs")}
				</pre>
				<p className="border-t border-border px-3 py-2 text-xs text-muted-foreground">
					{t(state && state.firstSeq > 1 ? "services.truncated" : "services.consoleHint")}
				</p>
			</section>
		</div>
	);
}
