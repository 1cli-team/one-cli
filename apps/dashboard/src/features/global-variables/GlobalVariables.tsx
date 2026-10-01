import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import useSWR from "swr";
import { Link } from "react-router-dom";
import {
	ArrowUp,
	Copy,
	Eye,
	EyeOff,
	Folder,
	FolderPlus,
	KeyRound,
	LockKeyhole,
	MoreHorizontal,
	Pencil,
	Plus,
	RefreshCw,
	SearchX,
	Settings2,
	ShieldCheck,
	Trash2,
} from "lucide-react";
import {
	createGlobalFolder,
	deleteGlobalSecret,
	getGlobalListing,
	getLocation,
	getProject,
	getSession,
	globalQuery,
	locationKey,
	message,
	readGlobalSecret,
	saveGlobalSecret,
	sessionKey,
	type GlobalLocation,
} from "@/api/session";
import { InfisicalBindingDialog } from "@/features/infisical-binding/InfisicalBindingDialog";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Field, FieldLabel } from "@/components/ui/field";
import { IconButton } from "@/components/ui/icon-button";
import {
	ErrorNotice,
	PageHeader,
	SearchInput,
	SectionHeading,
	StatePanel,
} from "@/components/ui/page-layout";
import { DiscardDialog } from "@/components/ui/discard-dialog";
import { Spinner } from "@/components/ui/spinner";
import { Skeleton } from "@/components/ui/skeleton";
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
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import {
	AlertDialog,
	AlertDialogCancel,
	AlertDialogContent,
	AlertDialogDescription,
	AlertDialogFooter,
	AlertDialogHeader,
	AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuItem,
	DropdownMenuSeparator,
	DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "@/components/ui/table";
import { useToast } from "@/hooks/useToast";

export function GlobalVariables() {
	const { t } = useTranslation();
	const session = useSWR(sessionKey, getSession, { refreshInterval: 2000 });
	const location = useSWR(locationKey, getLocation);
	const [configure, setConfigure] = useState(false);
	const current = location.data?.location;
	const signedIn = session.data?.session.loggedIn;
	const mismatched =
		current &&
		signedIn &&
		(current.userId !== session.data?.session.userId ||
			current.siteUrl !== session.data?.session.siteUrl ||
			(session.data?.session.organizationId &&
				current.organizationId !== session.data?.session.organizationId));
	return (
		<div className="mx-auto w-full max-w-6xl space-y-6">
			<PageHeader
				title={t("global.title")}
				description={t("global.description")}
				actions={
					signedIn && current ? (
						<Button variant="outline" onClick={() => setConfigure(true)}>
							<Settings2 />
							{t("binding.change")}
						</Button>
					) : undefined
				}
			/>
			{session.error || location.error ? (
				<ErrorNotice
					action={
						<Button
							variant="outline"
							size="sm"
							onClick={() => {
								void session.mutate();
								void location.mutate();
							}}
						>
							<RefreshCw />
							{t("secrets.retry")}
						</Button>
					}
				>
					{message(session.error || location.error)}
				</ErrorNotice>
			) : session.isLoading || location.isLoading ? (
				<div role="status" aria-label={t("session.loading")} className="space-y-4">
					<Skeleton className="h-24" />
					<Skeleton className="h-64" />
				</div>
			) : !signedIn ? (
				<Card>
					<StatePanel
						icon={LockKeyhole}
						title={t("session.signedOut")}
						description={t("global.loginRequired")}
					>
						<Button asChild>
							<Link to="/settings">{t("session.login")}</Link>
						</Button>
					</StatePanel>
				</Card>
			) : (
				<>
					{mismatched && <ErrorNotice>{t("global.mismatch")}</ErrorNotice>}
					{!current || mismatched ? (
						<Card>
							<StatePanel
								icon={KeyRound}
								title={t("binding.unbound")}
								description={t("binding.globalHint")}
							>
								<Button onClick={() => setConfigure(true)}>{t("binding.globalTitle")}</Button>
							</StatePanel>
						</Card>
					) : (
						<VariableBrowser
							key={`${session.data?.session.userId}:${current.siteUrl}:${current.projectId}`}
							location={current}
						/>
					)}
					<InfisicalBindingDialog
						key={`${session.data?.session.siteUrl}:${session.data?.session.userId}:${session.data?.session.organizationId}`}
						open={configure}
						onOpenChange={setConfigure}
						scope="global"
						initial={current ?? undefined}
					/>
				</>
			)}
		</div>
	);
}

function VariableBrowser({ location }: { location: GlobalLocation }) {
	const { t } = useTranslation();
	const toast = useToast();
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
	const pending = useRef(false);
	const actionTrigger = useRef<HTMLButtonElement | null>(null);
	const [editor, setEditor] = useState<{ key: string; value: string; existing: boolean } | null>(
		null,
	);
	const [deleting, setDeleting] = useState<string | null>(null);
	const [folder, setFolder] = useState<string | null>(null);
	const [busy, setBusy] = useState(false);
	const [error, setError] = useState("");
	const [showValue, setShowValue] = useState(false);
	const [discard, setDiscard] = useState(false);
	const dirty =
		Boolean(editor && (editor.value || (!editor.existing && editor.key))) || Boolean(folder);
	useEffect(() => {
		epoch.current++;
		setRevealed({});
		setEditor(null);
		setDeleting(null);
		setFolder(null);
		setError("");
		setSearch("");
		setShowValue(false);
		setDiscard(false);
		return () => {
			epoch.current++;
		};
	}, [query]);
	useEffect(() => {
		if (!dirty) return;
		const warn = (e: BeforeUnloadEvent) => e.preventDefault();
		window.addEventListener("beforeunload", warn);
		return () => window.removeEventListener("beforeunload", warn);
	}, [dirty]);
	async function action(fn: () => Promise<void>) {
		if (pending.current) return;
		pending.current = true;
		setBusy(true);
		setError("");
		const current = epoch.current;
		try {
			await fn();
		} catch (e) {
			if (current === epoch.current) setError(message(e));
		} finally {
			pending.current = false;
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
				toast.success(t("global.copied"));
			} else setRevealed((v) => ({ ...v, [key]: value }));
		});
	}
	function openEditor(key = "", existing = false) {
		if (!existing) actionTrigger.current = null;
		setError("");
		setShowValue(false);
		setEditor({ key, value: "", existing });
	}
	function closeEditor() {
		if (busy) return;
		if (dirty) setDiscard(true);
		else {
			setEditor(null);
			setFolder(null);
			setError("");
		}
	}
	const editing = editor !== null || folder !== null || deleting !== null;
	const variables =
		listing.data?.variables.filter((v) =>
			v.key.toLocaleLowerCase().includes(search.trim().toLocaleLowerCase()),
		) ?? [];
	const unavailable = busy || editing || listing.isLoading || Boolean(listing.error);
	return (
		<>
			<Card className="gap-0 overflow-hidden">
				<div className="border-b border-border p-5">
					<SectionHeading
						icon={ShieldCheck}
						title={location.projectName}
						description={t("global.browseHint")}
						actions={
							<div className="flex items-center gap-2">
								<Select
									value={environment}
									onValueChange={setEnvironment}
									disabled={editing || busy || detail.isLoading}
								>
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
								<IconButton
									label={t("global.refresh")}
									disabled={busy || listing.isValidating || editing}
									onClick={() => {
										void listing.mutate();
										void detail.mutate();
									}}
								>
									<RefreshCw className={listing.isValidating ? "animate-spin" : ""} />
								</IconButton>
							</div>
						}
					/>
				</div>
				{listing.error || detail.error || (error && !editing) ? (
					<div className="p-4">
						<ErrorNotice>{error || message(listing.error || detail.error)}</ErrorNotice>
					</div>
				) : null}
				<div className="grid ud-md:grid-cols-[180px_minmax(0,1fr)]">
					<nav
						aria-label={t("global.folders")}
						className="border-b border-border bg-muted/25 p-3 ud-md:border-r ud-md:border-b-0"
					>
						<div className="mb-3 flex items-center justify-between gap-2 px-1">
							<h3 className="text-xs font-medium text-muted-foreground">{t("global.folders")}</h3>
							<IconButton
								label={t("global.addFolder")}
								disabled={unavailable}
								onClick={() => {
									setError("");
									setFolder("");
								}}
							>
								<FolderPlus />
							</IconButton>
						</div>
						<div className="flex flex-wrap gap-1 ud-md:flex-col">
							{listing.data?.folders.map((f) => (
								<Button
									key={f}
									variant="ghost"
									className="justify-start"
									disabled={editing || busy}
									onClick={() => setPath(f)}
								>
									<Folder />
									<span className="truncate">{f.split("/").pop()}</span>
								</Button>
							))}
							{listing.data && !listing.data.folders.length && (
								<p className="px-1 text-xs text-muted-foreground">{t("global.noFolders")}</p>
							)}
						</div>
					</nav>
					<div className="min-w-0">
						<div className="flex flex-wrap items-center gap-2 border-b border-border px-4 py-3">
							<IconButton
								label={t("global.parent")}
								disabled={path === "/" || editing || busy}
								onClick={() => setPath(path.slice(0, path.lastIndexOf("/")) || "/")}
							>
								<ArrowUp />
							</IconButton>
							<Folder className="size-4 text-muted-foreground" aria-hidden="true" />
							<code className="min-w-0 flex-1 break-all text-xs">{path}</code>
							{listing.data && (
								<Badge variant="muted">
									{t("global.count", { count: listing.data.variables.length })}
								</Badge>
							)}
						</div>
						<div className="flex flex-wrap gap-3 p-4">
							<div className="min-w-40 flex-1">
								<SearchInput
									aria-label={t("global.search")}
									placeholder={t("global.search")}
									value={search}
									onChange={(e) => setSearch(e.target.value)}
								/>
							</div>
							<Button disabled={unavailable} onClick={() => openEditor()}>
								<Plus />
								{t("global.add")}
							</Button>
						</div>
						{listing.isLoading ? (
							<div role="status" aria-label={t("session.loading")} className="space-y-3 p-4">
								<Skeleton className="h-10" />
								<Skeleton className="h-10" />
								<Skeleton className="h-10" />
							</div>
						) : listing.data && variables.length > 0 ? (
							<Table>
								<TableHeader>
									<TableRow>
										<TableHead>{t("global.key")}</TableHead>
										<TableHead>{t("global.value")}</TableHead>
										<TableHead className="w-28 text-right">{t("global.actions")}</TableHead>
									</TableRow>
								</TableHeader>
								<TableBody>
									{variables.map((v) => (
										<TableRow key={v.key}>
											<TableCell className="max-w-64">
												<code className="break-all font-medium">{v.key}</code>
												{v.description && (
													<p className="mt-1 whitespace-pre-wrap text-xs text-muted-foreground">
														{v.description}
													</p>
												)}
											</TableCell>
											<TableCell className="max-w-64 break-all font-mono text-xs">
												{revealed[v.key] ?? "••••••••"}
											</TableCell>
											<TableCell>
												<div className="flex justify-end gap-1">
													<IconButton
														label={t(
															revealed[v.key] !== undefined ? "global.hide" : "global.reveal",
														)}
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
														{revealed[v.key] !== undefined ? <EyeOff /> : <Eye />}
													</IconButton>
													<IconButton
														label={t("global.copy")}
														disabled={busy}
														onClick={() => void read(v.key, true)}
													>
														<Copy />
													</IconButton>
													<DropdownMenu>
														<DropdownMenuTrigger asChild>
															<Button
																variant="ghost"
																size="icon"
																aria-label={t("global.more", { key: v.key })}
																onPointerDown={(event) => {
																	actionTrigger.current = event.currentTarget;
																}}
																onFocus={(event) => {
																	actionTrigger.current = event.currentTarget;
																}}
																disabled={busy}
															>
																<MoreHorizontal />
															</Button>
														</DropdownMenuTrigger>
														<DropdownMenuContent align="end">
															<DropdownMenuItem onSelect={() => openEditor(v.key, true)}>
																<Pencil />
																{t("global.edit")}
															</DropdownMenuItem>
															<DropdownMenuSeparator />
															<DropdownMenuItem
																variant="destructive"
																onSelect={() => {
																	setError("");
																	setDeleting(v.key);
																}}
															>
																<Trash2 />
																{t("global.delete")}
															</DropdownMenuItem>
														</DropdownMenuContent>
													</DropdownMenu>
												</div>
											</TableCell>
										</TableRow>
									))}
								</TableBody>
							</Table>
						) : listing.data && !listing.error ? (
							<StatePanel
								icon={search.trim() ? SearchX : KeyRound}
								title={t(search.trim() ? "global.noMatches" : "global.empty")}
								description={t(search.trim() ? "global.noMatchesHint" : "global.emptyHint")}
							>
								{search.trim() ? (
									<Button variant="outline" onClick={() => setSearch("")}>
										{t("workspaces.home.clearSearch")}
									</Button>
								) : (
									<Button variant="outline" onClick={() => openEditor()} disabled={unavailable}>
										<Plus />
										{t("global.add")}
									</Button>
								)}
							</StatePanel>
						) : null}
					</div>
				</div>
			</Card>
			<Dialog
				open={editor !== null}
				onOpenChange={(open) => {
					if (!open) closeEditor();
				}}
			>
				<DialogContent
					showCloseButton={!busy}
					onCloseAutoFocus={(event) => {
						if (actionTrigger.current?.isConnected) {
							event.preventDefault();
							actionTrigger.current.focus();
							actionTrigger.current = null;
						}
					}}
				>
					<DialogHeader>
						<DialogTitle>{editor?.existing ? t("global.edit") : t("global.add")}</DialogTitle>
						<DialogDescription>
							{location.projectName} · {environment} · {path}
						</DialogDescription>
					</DialogHeader>
					<form
						className="space-y-5"
						onSubmit={(e) => {
							e.preventDefault();
							if (!editor?.key.trim()) return;
							void action(async () => {
								await saveGlobalSecret(editor.key.trim(), editor.value, query, editor.existing);
								setRevealed({});
								setEditor(null);
								await listing.mutate();
								toast.success(t("global.saved"));
							});
						}}
					>
						<Field>
							<FieldLabel htmlFor="global-key">{t("global.key")}</FieldLabel>
							<Input
								id="global-key"
								className="font-mono"
								autoComplete="off"
								value={editor?.key ?? ""}
								readOnly={editor?.existing}
								disabled={busy}
								onChange={(e) => setEditor((v) => (v ? { ...v, key: e.target.value } : v))}
							/>
						</Field>
						<Field>
							<div className="flex items-center justify-between">
								<FieldLabel htmlFor="global-value">{t("global.newValue")}</FieldLabel>
								<IconButton
									label={t(showValue ? "global.hide" : "global.reveal")}
									onClick={() => setShowValue(!showValue)}
								>
									{showValue ? <EyeOff /> : <Eye />}
								</IconButton>
							</div>
							<Input
								id="global-value"
								type={showValue ? "text" : "password"}
								autoComplete="new-password"
								className="font-mono"
								value={editor?.value ?? ""}
								disabled={busy}
								onChange={(e) => setEditor((v) => (v ? { ...v, value: e.target.value } : v))}
							/>
						</Field>
						<p className="text-xs text-muted-foreground">{t("secrets.editorDescription")}</p>
						{error && <ErrorNotice>{error}</ErrorNotice>}
						<DialogFooter>
							<Button variant="outline" disabled={busy} onClick={closeEditor}>
								{t("form.cancel")}
							</Button>
							<Button type="submit" disabled={busy || !editor?.key.trim()}>
								{busy ? <Spinner /> : <ShieldCheck />}
								{t("global.saveRemote")}
							</Button>
						</DialogFooter>
					</form>
				</DialogContent>
			</Dialog>
			<AlertDialog
				open={deleting !== null}
				onOpenChange={(open) => {
					if (!open && !busy) setDeleting(null);
				}}
			>
				<AlertDialogContent
					onCloseAutoFocus={(event) => {
						if (actionTrigger.current?.isConnected) {
							event.preventDefault();
							actionTrigger.current.focus();
							actionTrigger.current = null;
						}
					}}
				>
					<AlertDialogHeader>
						<AlertDialogTitle>{t("global.delete")}</AlertDialogTitle>
						<AlertDialogDescription>
							{t("global.deleteHint", {
								key: deleting,
								project: location.projectName,
								environment,
								path,
							})}
						</AlertDialogDescription>
					</AlertDialogHeader>
					{error && <ErrorNotice>{error}</ErrorNotice>}
					<AlertDialogFooter>
						<AlertDialogCancel disabled={busy}>{t("form.cancel")}</AlertDialogCancel>
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
									toast.success(t("secrets.deleted"));
								})
							}
						>
							{busy ? <Spinner /> : <Trash2 />}
							{t("global.delete")}
						</Button>
					</AlertDialogFooter>
				</AlertDialogContent>
			</AlertDialog>
			<Dialog
				open={folder !== null}
				onOpenChange={(open) => {
					if (!open) closeEditor();
				}}
			>
				<DialogContent showCloseButton={!busy}>
					<DialogHeader>
						<DialogTitle>{t("global.addFolder")}</DialogTitle>
						<DialogDescription>
							{environment} · {path}
						</DialogDescription>
					</DialogHeader>
					<form
						className="space-y-5"
						onSubmit={(e) => {
							e.preventDefault();
							if (!folder?.trim()) return;
							void action(async () => {
								await createGlobalFolder(folder.trim(), query);
								setFolder(null);
								await listing.mutate();
								toast.success(t("global.folderCreated"));
							});
						}}
					>
						<Field>
							<FieldLabel htmlFor="global-folder">{t("global.folderName")}</FieldLabel>
							<Input
								id="global-folder"
								value={folder ?? ""}
								disabled={busy}
								onChange={(e) => setFolder(e.target.value)}
							/>
						</Field>
						{error && <ErrorNotice>{error}</ErrorNotice>}
						<DialogFooter>
							<Button variant="outline" disabled={busy} onClick={closeEditor}>
								{t("form.cancel")}
							</Button>
							<Button type="submit" disabled={busy || !folder?.trim()}>
								{busy ? <Spinner /> : <FolderPlus />}
								{t("global.saveRemote")}
							</Button>
						</DialogFooter>
					</form>
				</DialogContent>
			</Dialog>
			<DiscardDialog
				open={discard}
				onOpenChange={setDiscard}
				onDiscard={() => {
					setEditor(null);
					setFolder(null);
					setError("");
					setDiscard(false);
				}}
			/>
		</>
	);
}
