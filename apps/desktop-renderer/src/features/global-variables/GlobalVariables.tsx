import { useEnvironmentDirtyStore } from "@/features/environment-context/environment-dirty-store";
import { useEffect, useId, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import useSWR from "swr";
import { useSessionSWRConfig } from "@/lib/desktop/cache";
import { Link } from "react-router-dom";
import {
	Eye,
	EyeOff,
	FolderPlus,
	LockKeyhole,
	Plus,
	RefreshCw,
	SearchX,
	ShieldCheck,
	Trash2,
} from "lucide-react";
import {
	createGlobalFolder,
	deleteGlobalSecret,
	getGlobalTree,
	globalTreeKey,
	getProject,
	getSession,
	globalQuery,
	globalListingKey,
	message,
	remoteProjectsKey,
	readGlobalSecret,
	saveGlobalSecret,
	sessionKey,
	type GlobalLocation,
} from "@/api/session";
import { CredentialGroup, type CredentialActions } from "./CredentialTable";
import { SharedCredentialsNotice } from "@/features/infisical-session/SharedCredentialsNotice";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Field, FieldLabel } from "@/components/ui/field";
import { IconButton } from "@/components/ui/icon-button";
import { ErrorNotice, SearchInput, StatePanel } from "@/components/ui/page-layout";
import { SettingsPage } from "@/components/settings-layout";
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
	const account = session.data?.session;
	const preparation = session.data?.sharedCredentials;
	const current = preparation?.status === "ready" ? preparation.location : undefined;
	const usable =
		current &&
		account?.loggedIn &&
		current.userId === account.userId &&
		current.siteUrl === account.siteUrl &&
		current.organizationId === account.organizationId;
	return (
		<SettingsPage title={t("global.title")} className="h-full min-h-0">
			{session.error ? (
				<ErrorNotice
					action={
						<Button variant="outline" size="sm" onClick={() => void session.mutate()}>
							<RefreshCw />
							{t("secrets.retry")}
						</Button>
					}
				>
					{message(session.error)}
				</ErrorNotice>
			) : session.isLoading ? (
				<div
					role="status"
					aria-label={t("session.loading")}
					className="flex min-h-0 flex-1 flex-col gap-4"
				>
					<Skeleton className="h-20" />
					<Skeleton className="min-h-0 flex-1" />
				</div>
			) : !account?.loggedIn ? (
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
			) : usable ? (
				<VariableBrowser
					key={JSON.stringify([
						account.siteUrl,
						account.userId,
						account.organizationId,
						account.expiresAt,
						current.projectId,
					])}
					location={current}
				/>
			) : (
				<Card className="p-5">
					<SharedCredentialsNotice
						key={JSON.stringify([
							account.siteUrl,
							account.userId,
							account.organizationId,
							account.expiresAt,
						])}
						state={preparation}
						account={account}
					/>
				</Card>
			)}
		</SettingsPage>
	);
}

function VariableBrowser({ location }: { location: GlobalLocation }) {
	const { t } = useTranslation();
	const toast = useToast();
	const { mutate } = useSessionSWRConfig();
	const [environment, setEnvironment] = useState(location.defaultEnvironment);
	const [path, setPath] = useState("/");
	const [search, setSearch] = useState("");
	const [expanded, setExpanded] = useState<Record<string, boolean>>({});
	const detail = useSWR(
		remoteProjectsKey({ loggedIn: true, expired: false, ...location }, location.projectId),
		() => getProject(location.projectId),
	);
	const query = globalQuery(environment, path);
	const term = search.trim().toLocaleLowerCase();
	const searching = term.length > 0;
	const tree = useSWR(searching ? globalTreeKey(location, environment) : null, () =>
		getGlobalTree(environment),
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
	}, [environment]);
	const dirtyOwner = useId();
	const setDirty = useEnvironmentDirtyStore((state) => state.setDirty);
	const clearOwner = useEnvironmentDirtyStore((state) => state.clearOwner);
	useEffect(() => {
		setDirty(dirtyOwner, dirty, () => {
			setEditor(null);
			setFolder(null);
		});
		return () => clearOwner(dirtyOwner);
	}, [dirty, dirtyOwner, setDirty, clearOwner]);
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
	async function refresh() {
		setRevealed({});
		const prefix = `${globalListingKey(location, "")}?${new URLSearchParams({ env: environment })}&`;
		if (!searching) {
			await mutate(globalTreeKey(location, environment), undefined, { revalidate: false });
		}
		await mutate((key) => typeof key === "string" && key.startsWith(prefix));
	}
	async function read(folderPath: string, key: string, copy: boolean) {
		const scopedQuery = globalQuery(environment, folderPath);
		const identity = `${scopedQuery}:${key}`;
		if (!copy && revealed[identity] !== undefined) {
			setRevealed((current) => {
				const next = { ...current };
				delete next[identity];
				return next;
			});
			return;
		}
		const current = epoch.current;
		await action(async () => {
			const { value } = await readGlobalSecret(key, scopedQuery);
			if (current !== epoch.current) return;
			if (copy) {
				await navigator.clipboard.writeText(value);
				toast.success(t("global.copied"));
			} else setRevealed((values) => ({ ...values, [identity]: value }));
		});
	}
	function openEditor(folderPath: string, key = "", existing = false) {
		setPath(folderPath);
		setError("");
		setShowValue(false);
		setEditor({ key, value: "", existing });
	}
	function openFolder(folderPath: string) {
		setPath(folderPath);
		setError("");
		setFolder("");
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
	const unavailable = busy || editing;
	const matches = (tree.data ?? [])
		.map((listing) => ({
			...listing,
			folders: [],
			variables: listing.variables.filter(
				(variable) =>
					variable.key.toLocaleLowerCase().includes(term) ||
					listing.path.toLocaleLowerCase().includes(term),
			),
		}))
		.filter(
			(listing) => listing.variables.length > 0 || listing.path.toLocaleLowerCase().includes(term),
		);
	function expansionKey(folderPath: string) {
		return JSON.stringify([environment, searching ? "search" : "tree", folderPath]);
	}
	function isExpanded(folderPath: string) {
		return expanded[expansionKey(folderPath)] ?? (searching || folderPath === "/");
	}
	function toggle(folderPath: string) {
		const key = expansionKey(folderPath);
		setExpanded((current) => ({
			...current,
			[key]: !(current[key] ?? (searching || folderPath === "/")),
		}));
	}
	function expandPath(folderPath: string) {
		setExpanded((current) => {
			const next = { ...current };
			next[JSON.stringify([environment, "tree", "/"])] = true;
			let ancestor = "";
			for (const part of folderPath.split("/").filter(Boolean)) {
				ancestor += `/${part}`;
				next[JSON.stringify([environment, "tree", ancestor])] = true;
			}
			return next;
		});
	}
	const actions: CredentialActions = {
		read: (folderPath, key, copy) => {
			void read(folderPath, key, copy);
		},
		edit: (folderPath, key) => openEditor(folderPath, key, true),
		remove: (folderPath, key) => {
			setPath(folderPath);
			setError("");
			setDeleting(key);
		},
		addVariable: (folderPath) => openEditor(folderPath),
		addFolder: openFolder,
	};
	const groupProps = {
		location,
		environment,
		isExpanded,
		toggle,
		actions,
		revealed,
		disabled: unavailable,
		actionTrigger,
	};
	return (
		<>
			<div className="flex min-h-0 flex-1 flex-col gap-4">
				<div className="flex shrink-0 flex-wrap items-center gap-3">
					<Select
						value={environment}
						onValueChange={setEnvironment}
						disabled={unavailable || detail.isLoading}
					>
						<SelectTrigger className="w-48" aria-label={t("global.environment")}>
							<SelectValue />
						</SelectTrigger>
						<SelectContent>
							{detail.data?.environments.map((item) => (
								<SelectItem key={item.slug} value={item.slug}>
									{item.name} ({item.slug})
								</SelectItem>
							))}
						</SelectContent>
					</Select>
					<div className="min-w-48 max-w-sm flex-1">
						<SearchInput
							aria-label={t("global.searchAll")}
							placeholder={t("global.searchAll")}
							value={search}
							onChange={(event) => setSearch(event.target.value)}
							disabled={editing}
						/>
					</div>
					<div className="ml-auto flex items-center gap-2">
						<IconButton
							label={t("global.refresh")}
							disabled={unavailable}
							onClick={() =>
								void action(async () => {
									await Promise.all([refresh(), detail.mutate()]);
								})
							}
						>
							<RefreshCw />
						</IconButton>
						<IconButton
							label={t("global.addFolder")}
							disabled={unavailable}
							onClick={(event) => {
								actionTrigger.current = event.currentTarget;
								openFolder("/");
							}}
						>
							<FolderPlus />
						</IconButton>
						<Button
							disabled={unavailable}
							onClick={(event) => {
								actionTrigger.current = event.currentTarget;
								openEditor("/");
							}}
						>
							<Plus />
							{t("global.add")}
						</Button>
					</div>
				</div>
				{(detail.error || (error && !editing)) && (
					<ErrorNotice>{error || message(detail.error)}</ErrorNotice>
				)}
				<Table
					containerClassName="min-h-0 overflow-auto rounded-lg border border-border bg-card"
					className="min-w-[32rem] table-fixed"
				>
					<colgroup>
						<col className="w-1/2" />
						<col />
						<col className="w-28" />
					</colgroup>
					<TableHeader className="sticky top-0 z-10 bg-card">
						<TableRow>
							<TableHead className="px-4">{t("global.name")}</TableHead>
							<TableHead className="px-4">{t("global.value")}</TableHead>
							<TableHead className="px-4 text-right">{t("global.actions")}</TableHead>
						</TableRow>
					</TableHeader>
					<TableBody>
						{searching ? (
							tree.isLoading ? (
								<TableRow>
									<TableCell colSpan={3}>
										<span
											role="status"
											className="flex items-center gap-2 py-4 text-muted-foreground"
										>
											<Spinner />
											{t("global.searching")}
										</span>
									</TableCell>
								</TableRow>
							) : tree.error ? (
								<TableRow>
									<TableCell colSpan={3}>
										<ErrorNotice
											action={
												<Button variant="outline" size="sm" onClick={() => void tree.mutate()}>
													{t("secrets.retry")}
												</Button>
											}
										>
											{message(tree.error)}
										</ErrorNotice>
									</TableCell>
								</TableRow>
							) : matches.length > 0 ? (
								matches.map((listing) => (
									<CredentialGroup
										{...groupProps}
										key={`${environment}:search:${listing.path}`}
										path={listing.path}
										depth={0}
										listing={listing}
									/>
								))
							) : (
								<TableRow>
									<TableCell colSpan={3}>
										<StatePanel icon={SearchX} title={t("global.noMatches")}>
											<Button variant="outline" onClick={() => setSearch("")}>
												{t("workspaces.home.clearSearch")}
											</Button>
										</StatePanel>
									</TableCell>
								</TableRow>
							)
						) : (
							<CredentialGroup {...groupProps} key={`${environment}:tree`} path="/" depth={0} />
						)}
					</TableBody>
				</Table>
				<p className="shrink-0 text-xs text-muted-foreground">
					{t("global.project")} · <span className="font-mono">{location.projectName}</span>
				</p>
			</div>
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
						className="flex flex-col gap-5"
						onSubmit={(e) => {
							e.preventDefault();
							if (!editor?.key.trim()) return;
							void action(async () => {
								await saveGlobalSecret(editor.key.trim(), editor.value, query, editor.existing);
								expandPath(path);
								setRevealed({});
								setEditor(null);
								await refresh();
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
									await refresh();
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
						<DialogTitle>{t("global.addFolder")}</DialogTitle>
						<DialogDescription>
							{environment} · {path}
						</DialogDescription>
					</DialogHeader>
					<form
						className="flex flex-col gap-5"
						onSubmit={(e) => {
							e.preventDefault();
							if (!folder?.trim()) return;
							void action(async () => {
								await createGlobalFolder(folder.trim(), query);
								expandPath(path);
								setFolder(null);
								await refresh();
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
