import { DiscardDialog } from "@/components/ui/discard-dialog";
import { Database, FolderPlus, RefreshCw, Save } from "lucide-react";
import { ErrorNotice, SectionHeading } from "@/components/ui/page-layout";
import { Spinner } from "@/components/ui/spinner";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import useSWR, { useSWRConfig } from "swr";
import {
	bindLocation,
	createRemoteProject,
	getProject,
	getProjects,
	initializeGlobalLocation,
	message,
	type GlobalLocation,
	type RemoteProject,
} from "@/api/session";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";

export function LocationPicker({
	initial,
	onSaved,
	onCancel,
	onBusyChange,
}: {
	initial?: GlobalLocation;
	onSaved: (location?: GlobalLocation) => Promise<void>;
	onCancel?: () => void;
	onBusyChange?: (busy: boolean) => void;
}) {
	const { t } = useTranslation();
	const { mutate } = useSWRConfig();
	const projects = useSWR("/infisical/projects", getProjects);
	const [project, setProject] = useState(initial?.projectId ?? "");
	const [environment, setEnvironment] = useState(initial?.defaultEnvironment ?? "");
	const detail = useSWR(project ? `/infisical/projects/${project}` : null, () =>
		getProject(project),
	);
	const [busy, setBusy] = useState(false);
	const [error, setError] = useState("");
	const [creating, setCreating] = useState(false);
	const [name, setName] = useState("");
	const [createError, setCreateError] = useState("");
	const [discard, setDiscard] = useState(false);
	useEffect(() => {
		onBusyChange?.(busy);
	}, [busy, onBusyChange]);
	function closeCreation() {
		if (busy) return;
		if (name.trim()) setDiscard(true);
		else setCreating(false);
	}

	async function save(useDefault: boolean) {
		if (busy) return;
		setBusy(true);
		setError("");
		try {
			const result = useDefault
				? await initializeGlobalLocation()
				: await bindLocation(project, environment);
			await onSaved(result.location);
		} catch (e) {
			setError(message(e));
			// Setup may have created the remote project before a later step failed.
			void projects.mutate().catch(() => undefined);
		} finally {
			setBusy(false);
		}
	}
	async function create() {
		if (busy || !name.trim()) return;
		setBusy(true);
		setCreateError("");
		try {
			const created = await createRemoteProject(name.trim());
			await mutate(`/infisical/projects/${created.id}`, created, { revalidate: false });
			await projects.mutate(
				(current: RemoteProject[] | undefined) =>
					[...(current ?? []).filter((p) => p.id !== created.id), created].sort((a, b) =>
						a.name.localeCompare(b.name),
					),
				{ revalidate: false },
			);
			setProject(created.id);
			setEnvironment(created.environments.some((e) => e.slug === "dev") ? "dev" : "");
			setError("");
			setCreating(false);
		} catch (e) {
			setCreateError(message(e));
			void projects.mutate().catch(() => undefined);
		} finally {
			setBusy(false);
		}
	}
	return (
		<>
			<Card>
				<CardContent className="space-y-6 p-6">
					<SectionHeading
						icon={Database}
						title={t("global.location")}
						description={t("global.locationHint")}
					/>
					{!initial && !onBusyChange ? (
						<div className="flex flex-wrap items-center justify-between gap-4 rounded-lg border bg-muted/40 p-4">
							<div className="space-y-1">
								<h3 className="font-medium">{t("global.defaultLocation")}</h3>
								<p className="font-mono text-sm">shared-credentials / dev /</p>
								<p className="text-sm text-muted-foreground">{t("global.defaultLocationHint")}</p>
							</div>
							<Button disabled={busy} onClick={() => void save(true)}>
								{busy ? t("global.saving") : t("global.useDefault")}
							</Button>
						</div>
					) : null}

					<div className="grid gap-6 ud-md:grid-cols-2">
						<div className="space-y-2">
							<Label>{t("global.project")}</Label>
							<div className="flex gap-2">
								<Select
									value={project}
									onValueChange={(v) => {
										setProject(v);
										setEnvironment("");
									}}
									disabled={busy || projects.isLoading}
								>
									<SelectTrigger className="min-w-0 flex-1" aria-label={t("global.project")}>
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
								<Button
									variant="outline"
									disabled={busy}
									onClick={() => {
										setName("");
										setCreateError("");
										setCreating(true);
									}}
								>
									<FolderPlus />
									{t("global.createProject")}
								</Button>
							</div>
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
						<p className="text-sm text-muted-foreground">{t("global.noProjects")}</p>
					) : null}
					{error || projects.error || detail.error ? (
						<ErrorNotice
							action={
								projects.error || detail.error ? (
									<Button
										variant="outline"
										size="sm"
										onClick={() => {
											void projects.mutate();
											void detail.mutate();
										}}
									>
										<RefreshCw />
										{t("secrets.retry")}
									</Button>
								) : undefined
							}
						>
							{error || message(projects.error || detail.error)}
						</ErrorNotice>
					) : null}
					<div className="flex flex-row-reverse justify-start gap-2 border-t border-border pt-5">
						<Button
							disabled={busy || !project || !environment || !detail.data || !!detail.error}
							onClick={() => void save(false)}
						>
							{busy ? <Spinner /> : <Save />}
							{busy
								? t("global.saving")
								: t(onBusyChange ? "binding.bindExisting" : "global.saveLocation")}
						</Button>
						{onCancel ? (
							<Button variant="outline" disabled={busy} onClick={onCancel}>
								{t("session.cancel")}
							</Button>
						) : null}
					</div>
				</CardContent>
			</Card>
			<Dialog
				open={creating}
				onOpenChange={(open) => {
					if (!open) closeCreation();
				}}
			>
				<DialogContent>
					<DialogHeader>
						<DialogTitle>{t("global.createProject")}</DialogTitle>
						<DialogDescription>{t("global.createProjectHint")}</DialogDescription>
					</DialogHeader>
					<form
						className="space-y-4"
						onSubmit={(e) => {
							e.preventDefault();
							if (!busy && name.trim()) void create();
						}}
					>
						<div className="space-y-2">
							<Label htmlFor="shared-project-name">{t("global.projectName")}</Label>
							<Input
								id="shared-project-name"
								autoFocus
								maxLength={64}
								value={name}
								onChange={(e) => setName(e.target.value)}
								disabled={busy}
								placeholder="shared-credentials"
							/>
						</div>
						{createError ? (
							<p role="alert" className="text-sm text-error-foreground">
								{createError}
							</p>
						) : null}
						<DialogFooter>
							<Button type="button" variant="outline" disabled={busy} onClick={closeCreation}>
								{t("session.cancel")}
							</Button>
							<Button type="submit" disabled={busy || !name.trim()}>
								{busy ? t("global.creatingProject") : t("global.createAndSelect")}
							</Button>
						</DialogFooter>
					</form>
				</DialogContent>
			</Dialog>
			<DiscardDialog
				open={discard}
				onOpenChange={setDiscard}
				onDiscard={() => {
					setCreating(false);
					setDiscard(false);
					setName("");
				}}
			/>
		</>
	);
}
