import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import useSWR from "swr";
import { Link } from "react-router-dom";
import {
	bindLocation,
	createGlobalFolder,
	deleteGlobalSecret,
	getGlobalListing,
	getLocation,
	getProject,
	getProjects,
	getSession,
	globalQuery,
	locationKey,
	message,
	readGlobalSecret,
	saveGlobalSecret,
	sessionKey,
	type GlobalLocation,
} from "@/api/session";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "@/components/ui/table";

export function GlobalVariables() {
	const { t } = useTranslation();
	const session = useSWR(sessionKey, getSession, { refreshInterval: 2000 });
	const location = useSWR(locationKey, getLocation);
	const [configure, setConfigure] = useState(false);
	if (session.error || location.error)
		return <p role="alert">{message(session.error || location.error)}</p>;
	if (session.isLoading || location.isLoading) return <p role="status">{t("session.loading")}</p>;
	if (!session.data?.session.loggedIn)
		return (
			<Card>
				<CardContent className="space-y-4 p-6">
					<h1 className="text-lg font-semibold">{t("global.title")}</h1>
					<p>{t("global.loginRequired")}</p>
					<Button asChild>
						<Link to="/settings">{t("session.login")}</Link>
					</Button>
				</CardContent>
			</Card>
		);
	const current = location.data?.location;
	const mismatched =
		current &&
		(current.userId !== session.data.session.userId ||
			current.siteUrl !== session.data.session.siteUrl ||
			(session.data.session.organizationId &&
				current.organizationId !== session.data.session.organizationId));
	return (
		<div className="space-y-4">
			<div className="flex flex-wrap items-center justify-between gap-3">
				<div>
					<h1 className="text-xl font-semibold">{t("global.title")}</h1>
					<p className="mt-1 text-sm text-muted-foreground">{t("global.description")}</p>
				</div>
				<Button variant="outline" onClick={() => setConfigure(true)}>
					{t("global.location")}
				</Button>
			</div>
			{mismatched ? <p role="alert">{t("global.mismatch")}</p> : null}
			{!current || configure || mismatched ? (
				<LocationPicker
					initial={current ?? undefined}
					onSaved={async () => {
						await location.mutate();
						setConfigure(false);
					}}
					onCancel={current && !mismatched ? () => setConfigure(false) : undefined}
				/>
			) : (
				<VariableBrowser
					key={`${session.data.session.userId}:${current.siteUrl}:${current.projectId}`}
					location={current}
				/>
			)}
		</div>
	);
}
function LocationPicker({
	initial,
	onSaved,
	onCancel,
}: {
	initial?: GlobalLocation;
	onSaved: () => Promise<void>;
	onCancel?: () => void;
}) {
	const { t } = useTranslation();
	const projects = useSWR("/infisical/projects", getProjects);
	const [project, setProject] = useState(initial?.projectId ?? "");
	const [environment, setEnvironment] = useState(initial?.defaultEnvironment ?? "");
	const detail = useSWR(project ? `/infisical/projects/${project}` : null, () =>
		getProject(project),
	);
	const [busy, setBusy] = useState(false);
	const [error, setError] = useState("");
	return (
		<Card>
			<CardContent className="space-y-4 p-5">
				<h2 className="font-semibold">{t("global.location")}</h2>
				<p className="text-sm text-muted-foreground">{t("global.locationHint")}</p>
				<div className="grid gap-4 sm:grid-cols-2">
					<div className="space-y-2">
						<Label>{t("global.project")}</Label>
						<Select
							value={project}
							onValueChange={(v) => {
								setProject(v);
								setEnvironment("");
							}}
							disabled={busy}
						>
							<SelectTrigger aria-label={t("global.project")}>
								<SelectValue placeholder={t("global.selectProject")} />
							</SelectTrigger>
							<SelectContent>
								{projects.data?.map((p) => (
									<SelectItem key={p.id} value={p.id}>
										{p.name}
									</SelectItem>
								))}
							</SelectContent>
						</Select>
					</div>
					<div className="space-y-2">
						<Label>{t("global.defaultEnv")}</Label>
						<Select
							value={environment}
							onValueChange={setEnvironment}
							disabled={busy || !detail.data}
						>
							<SelectTrigger aria-label={t("global.defaultEnv")}>
								<SelectValue placeholder={t("global.selectEnv")} />
							</SelectTrigger>
							<SelectContent>
								{detail.data?.environments.map((e) => (
									<SelectItem key={e.slug} value={e.slug}>
										{e.name} ({e.slug})
									</SelectItem>
								))}
							</SelectContent>
						</Select>
					</div>
				</div>
				{projects.data?.length === 0 ? (
					<p>
						{t("global.noProjects")}{" "}
						<a
							className="underline"
							href={initial?.siteUrl || "https://app.infisical.com"}
							target="_blank"
							rel="noreferrer"
						>
							Infisical
						</a>
					</p>
				) : null}
				{error || projects.error || detail.error ? (
					<p role="alert" className="text-sm text-error-foreground">
						{error || message(projects.error || detail.error)}
					</p>
				) : null}
				<div className="flex gap-2">
					<Button
						disabled={busy || !project || !environment}
						onClick={async () => {
							setBusy(true);
							setError("");
							try {
								await bindLocation(project, environment);
								await onSaved();
							} catch (e) {
								setError(message(e));
							} finally {
								setBusy(false);
							}
						}}
					>
						{t("global.saveLocation")}
					</Button>
					{onCancel ? (
						<Button variant="outline" onClick={onCancel}>
							{t("session.cancel")}
						</Button>
					) : null}
				</div>
			</CardContent>
		</Card>
	);
}
function VariableBrowser({ location }: { location: GlobalLocation }) {
	const { t } = useTranslation();
	const [environment, setEnvironment] = useState(location.defaultEnvironment);
	const [path, setPath] = useState("/");
	const [search, setSearch] = useState("");
	const detail = useSWR(`/infisical/projects/${location.projectId}`, () =>
		getProject(location.projectId),
	);
	const query = globalQuery(environment, path);
	const listing = useSWR(
		`/global-env/secrets:${location.userId}:${location.siteUrl}:${location.projectId}${query}`,
		() => getGlobalListing(query),
	);
	const [revealed, setRevealed] = useState<Record<string, string>>({});
	const epoch = useRef(0);
	const [editor, setEditor] = useState<{ key: string; value: string; existing: boolean } | null>(
		null,
	);
	const [deleting, setDeleting] = useState<string | null>(null);
	const [folder, setFolder] = useState<string | null>(null);
	const [busy, setBusy] = useState(false);
	const [error, setError] = useState("");
	useEffect(() => {
		epoch.current++;
		setRevealed({});
		setEditor(null);
		setDeleting(null);
		setFolder(null);
		setError("");
		return () => {
			epoch.current++;
		};
	}, [query]);
	useEffect(() => {
		if (!editor) return;
		const warn = (e: BeforeUnloadEvent) => e.preventDefault();
		window.addEventListener("beforeunload", warn);
		return () => window.removeEventListener("beforeunload", warn);
	}, [editor]);
	async function action(fn: () => Promise<void>) {
		setBusy(true);
		setError("");
		try {
			await fn();
		} catch (e) {
			setError(message(e));
		} finally {
			setBusy(false);
		}
	}
	async function read(key: string, copy: boolean) {
		const current = epoch.current;
		await action(async () => {
			const { value } = await readGlobalSecret(key, query);
			if (current !== epoch.current) return;
			if (copy) {
				await navigator.clipboard.writeText(value);
			} else setRevealed((v) => ({ ...v, [key]: value }));
		});
	}
	const editing = editor !== null || folder !== null || deleting !== null;
	return (
		<Card>
			<CardContent className="space-y-4 p-5">
				<div className="flex flex-wrap items-center justify-between gap-3">
					<div>
						<h2 className="font-semibold">{location.projectName}</h2>
						<p className="mt-1 text-xs text-muted-foreground">{t("global.browseHint")}</p>
					</div>
					<Select value={environment} onValueChange={setEnvironment} disabled={editing || busy}>
						<SelectTrigger className="w-48" aria-label={t("global.environment")}>
							<SelectValue />
						</SelectTrigger>
						<SelectContent>
							{detail.data?.environments.map((e) => (
								<SelectItem key={e.slug} value={e.slug}>
									{e.name} ({e.slug})
								</SelectItem>
							))}
						</SelectContent>
					</Select>
				</div>
				<div className="flex flex-wrap items-center gap-2">
					<Button
						variant="outline"
						disabled={path === "/" || editing || busy}
						onClick={() => setPath(path.slice(0, path.lastIndexOf("/")) || "/")}
					>
						↑ {t("global.parent")}
					</Button>
					<code className="break-all text-sm">{path}</code>
					<Button variant="ghost" disabled={busy} onClick={() => void listing.mutate()}>
						{t("global.refresh")}
					</Button>
				</div>
				{listing.error || detail.error || error ? (
					<p role="alert" className="text-sm text-error-foreground">
						{error || message(listing.error || detail.error)}
					</p>
				) : null}
				{listing.isLoading ? <p role="status">{t("session.loading")}</p> : null}
				<div className="grid gap-4 md:grid-cols-[200px_minmax(0,1fr)]">
					<nav
						aria-label={t("global.folders")}
						className="space-y-1 rounded-md border border-border p-2"
					>
						<p className="p-2 text-xs text-muted-foreground">{t("global.folders")}</p>
						{listing.data?.folders.map((f) => (
							<Button
								key={f}
								variant="ghost"
								className="w-full justify-start overflow-hidden"
								disabled={editing || busy}
								onClick={() => setPath(f)}
							>
								<span className="truncate">{f.split("/").pop()}</span>
							</Button>
						))}
						<Button
							variant="outline"
							className="mt-2 w-full"
							disabled={busy || !!listing.error}
							onClick={() => setFolder("")}
						>
							{t("global.addFolder")}
						</Button>
					</nav>
					<div className="min-w-0 space-y-3">
						<div className="flex gap-2">
							<Input
								aria-label={t("global.search")}
								placeholder={t("global.search")}
								value={search}
								onChange={(e) => setSearch(e.target.value)}
							/>
							<Button
								disabled={busy || !!listing.error}
								onClick={() => setEditor({ key: "", value: "", existing: false })}
							>
								{t("global.add")}
							</Button>
						</div>
						<Table>
							<TableHeader>
								<TableRow>
									<TableHead>{t("global.key")}</TableHead>
									<TableHead>{t("global.value")}</TableHead>
									<TableHead>{t("global.actions")}</TableHead>
								</TableRow>
							</TableHeader>
							<TableBody>
								{listing.data?.variables
									.filter((v) => v.key.toLowerCase().includes(search.toLowerCase()))
									.map((v) => (
										<TableRow key={v.key}>
											<TableCell className="max-w-64">
												<code className="break-all">{v.key}</code>
												{v.description ? (
													<p className="mt-1 whitespace-pre-wrap text-xs text-muted-foreground">
														{v.description}
													</p>
												) : null}
											</TableCell>
											<TableCell className="max-w-64 break-all font-mono">
												{revealed[v.key] ?? "••••••••"}
											</TableCell>
											<TableCell>
												<div className="flex flex-wrap gap-1">
													<Button
														size="sm"
														variant="ghost"
														disabled={busy}
														onClick={() =>
															revealed[v.key] !== undefined
																? setRevealed((r) => {
																		const next = { ...r };
																		delete next[v.key];
																		return next;
																	})
																: void read(v.key, false)
														}
													>
														{revealed[v.key] !== undefined ? t("global.hide") : t("global.reveal")}
													</Button>
													<Button
														size="sm"
														variant="ghost"
														disabled={busy}
														onClick={() => void read(v.key, true)}
													>
														{t("global.copy")}
													</Button>
													<Button
														size="sm"
														variant="ghost"
														disabled={busy}
														onClick={() => setEditor({ key: v.key, value: "", existing: true })}
													>
														{t("global.edit")}
													</Button>
													<Button
														size="sm"
														variant="ghost"
														disabled={busy}
														onClick={() => setDeleting(v.key)}
													>
														{t("global.delete")}
													</Button>
												</div>
											</TableCell>
										</TableRow>
									))}
							</TableBody>
						</Table>
						{listing.data?.variables.length === 0 ? (
							<p className="py-5 text-center text-sm text-muted-foreground">{t("global.empty")}</p>
						) : null}
					</div>
				</div>
				<Dialog
					open={editor !== null}
					onOpenChange={(open) => {
						if (!open && !busy && !editor?.value) setEditor(null);
					}}
				>
					<DialogContent>
						<DialogHeader>
							<DialogTitle>{editor?.existing ? t("global.edit") : t("global.add")}</DialogTitle>
							<DialogDescription>
								{location.projectName} · {environment} · {path}
							</DialogDescription>
						</DialogHeader>
						<Label htmlFor="global-key">{t("global.key")}</Label>
						<Input
							id="global-key"
							value={editor?.key ?? ""}
							disabled={editor?.existing || busy}
							onChange={(e) => setEditor((v) => (v ? { ...v, key: e.target.value } : v))}
						/>
						<Label htmlFor="global-value">{t("global.newValue")}</Label>
						<Input
							id="global-value"
							type="password"
							autoComplete="new-password"
							value={editor?.value ?? ""}
							disabled={busy}
							onChange={(e) => setEditor((v) => (v ? { ...v, value: e.target.value } : v))}
						/>
						{error ? <p role="alert">{error}</p> : null}
						<Button
							disabled={busy || !editor?.key}
							onClick={() =>
								void action(async () => {
									if (!editor) return;
									await saveGlobalSecret(editor.key, editor.value, query, editor.existing);
									setRevealed({});
									setEditor(null);
									await listing.mutate();
								})
							}
						>
							{t("global.saveRemote")}
						</Button>
						<Button variant="outline" disabled={busy} onClick={() => setEditor(null)}>
							{t("global.discard")}
						</Button>
					</DialogContent>
				</Dialog>
				<Dialog
					open={deleting !== null}
					onOpenChange={(open) => {
						if (!open && !busy) setDeleting(null);
					}}
				>
					<DialogContent>
						<DialogHeader>
							<DialogTitle>{t("global.delete")}</DialogTitle>
							<DialogDescription>
								{t("global.deleteHint", {
									key: deleting,
									project: location.projectName,
									environment,
									path,
								})}
							</DialogDescription>
						</DialogHeader>
						{error ? <p role="alert">{error}</p> : null}
						<Button
							variant="destructive"
							disabled={busy}
							onClick={() =>
								void action(async () => {
									if (!deleting) return;
									await deleteGlobalSecret(deleting, query);
									setRevealed({});
									setDeleting(null);
									await listing.mutate();
								})
							}
						>
							{t("global.delete")}
						</Button>
						<Button variant="outline" disabled={busy} onClick={() => setDeleting(null)}>
							{t("session.cancel")}
						</Button>
					</DialogContent>
				</Dialog>
				<Dialog
					open={folder !== null}
					onOpenChange={(open) => {
						if (!open && !busy) setFolder(null);
					}}
				>
					<DialogContent>
						<DialogHeader>
							<DialogTitle>{t("global.addFolder")}</DialogTitle>
							<DialogDescription>
								{environment} · {path}
							</DialogDescription>
						</DialogHeader>
						<Input
							aria-label={t("global.folderName")}
							value={folder ?? ""}
							onChange={(e) => setFolder(e.target.value)}
						/>
						{error ? <p role="alert">{error}</p> : null}
						<Button
							disabled={busy || !folder}
							onClick={() =>
								void action(async () => {
									await createGlobalFolder(folder!, query);
									setFolder(null);
									await listing.mutate();
								})
							}
						>
							{t("global.saveRemote")}
						</Button>
					</DialogContent>
				</Dialog>
			</CardContent>
		</Card>
	);
}
