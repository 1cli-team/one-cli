import { useState } from "react";
import useSWR from "swr";
import { desktop } from "./runtime";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { SettingsDetails, SettingsPage, SettingsSection } from "@/components/settings-layout";
import { Skeleton } from "@/components/ui/skeleton";
import {
	Dialog,
	DialogContent,
	DialogHeader,
	DialogTitle,
	DialogDescription,
} from "@/components/ui/dialog";
import { ErrorNotice } from "@/components/ui/page-layout";
import { http } from "@/lib/http";
interface Installation {
	target: string;
	version: string;
	bundledVersion: string;
	action: string;
	digest: string;
	installed: boolean;
	pathReady: boolean;
	resolvedPath: string;
	pathHint: string;
	configPath: string;
}
interface Update {
	enabled: boolean;
	version?: string;
	notes?: string;
	reason?: string;
}
export function DesktopSettings() {
	const { t } = useTranslation();
	return (
		<SettingsPage title={t("desktop.settings")}>
			<SettingsBody />
		</SettingsPage>
	);
}
function SettingsBody({ onDone }: { onDone?: () => void }) {
	const { t } = useTranslation();
	const state = useSWR("/desktop/installation", () =>
		http.get<Installation>("/desktop/installation"),
	);
	const [busy, setBusy] = useState(false);
	const [installationError, setInstallationError] = useState("");
	const [updateError, setUpdateError] = useState("");
	const [update, setUpdate] = useState<Update>();
	const s = state.data;
	async function action(work: () => Promise<unknown>, setError: (message: string) => void) {
		setBusy(true);
		setError("");
		try {
			await work();
		} catch (e) {
			setError((e as Error).message || String(e));
		} finally {
			setBusy(false);
		}
	}
	return (
		<div className="flex flex-col gap-8">
			<SettingsSection title={t("desktop.terminal")}>
				{s ? (
					<>
						<SettingsDetails>
							<dt>{t("desktop.bundledVersion")}</dt>
							<dd className="font-mono">{s.bundledVersion}</dd>
							<dt>{t("desktop.target")}</dt>
							<dd className="font-mono">{s.target}</dd>
							<dt>{t("desktop.existingVersion")}</dt>
							<dd className="font-mono">{s.version || t("desktop.notInstalled")}</dd>
							<dt>{t("desktop.configuration")}</dt>
							<dd className="font-mono">{s.configPath}</dd>
						</SettingsDetails>
						{!s.pathReady ? (
							<div className="flex flex-col gap-2 rounded-md bg-muted/50 p-3 text-sm">
								<p>{t("desktop.pathPending")}</p>
								<p className="break-all">{s.pathHint}</p>
								{s.resolvedPath && (
									<p className="break-all">{t("desktop.resolvedPath", { path: s.resolvedPath })}</p>
								)}
							</div>
						) : (
							<p className="text-sm text-muted-foreground">{t("desktop.pathDetected")}</p>
						)}
						<div className="flex flex-wrap items-center justify-between gap-4 border-t border-border pt-4">
							<p className="min-w-0 flex-1 text-sm text-muted-foreground">
								{t(`desktop.installAction.${s.action}`)}
							</p>
							<div className="flex flex-wrap items-center justify-end gap-2">
								{["install", "upgrade"].includes(s.action) && (
									<Button
										disabled={busy}
										onClick={() =>
											void action(async () => {
												await http.post(
													"/desktop/installation",
													{ target: s.target, digest: s.digest },
													{ timeout: 30000 },
												);
												await state.mutate();
											}, setInstallationError)
										}
									>
										{busy ? t("desktop.working") : t("desktop.install")}
									</Button>
								)}
								<Button
									variant="outline"
									disabled={busy || state.isValidating}
									onClick={() => void state.mutate()}
								>
									{t("desktop.refresh")}
								</Button>
							</div>
						</div>
					</>
				) : state.isLoading ? (
					<div role="status" aria-label={t("desktop.working")} className="flex flex-col gap-3">
						<Skeleton className="h-5 w-48" />
						<Skeleton className="h-20" />
					</div>
				) : null}
				{(installationError || state.error) && (
					<ErrorNotice>
						{installationError
							? t(`desktop.native.${installationError}`, { defaultValue: installationError })
							: state.error.message}
					</ErrorNotice>
				)}
			</SettingsSection>
			{!onDone && (
				<SettingsSection title={t("desktop.updates")}>
					<div className="flex flex-wrap items-center justify-between gap-4">
						<p className="min-w-0 flex-1 text-sm text-muted-foreground">
							{update
								? update.enabled
									? update.version
										? t("desktop.updateAvailable", { version: update.version })
										: t("desktop.upToDate")
									: t("desktop.updateUnconfigured")
								: t("desktop.updateNotChecked")}
						</p>
						<Button
							disabled={busy}
							variant="outline"
							onClick={() =>
								void action(async () => setUpdate(await desktop().checkUpdate()), setUpdateError)
							}
						>
							{t("desktop.checkUpdate")}
						</Button>
					</div>
					{update && (update.notes || update.version) && (
						<div className="flex flex-col gap-3 border-t border-border pt-4">
							{update.notes && (
								<p className="whitespace-pre-wrap text-sm text-muted-foreground">{update.notes}</p>
							)}
							{update.version && (
								<div className="flex justify-end">
									<Button
										disabled={busy}
										onClick={() => void action(() => desktop().installUpdate(), setUpdateError)}
									>
										{t("desktop.installUpdate")}
									</Button>
								</div>
							)}
						</div>
					)}
					{updateError && (
						<ErrorNotice>
							{t(`desktop.native.${updateError}`, { defaultValue: updateError })}
						</ErrorNotice>
					)}
				</SettingsSection>
			)}
			{onDone && (
				<div className="flex justify-end">
					<Button variant="outline" disabled={busy} onClick={onDone}>
						{t(s?.installed ? "desktop.continue" : "desktop.skipTerminal")}
					</Button>
				</div>
			)}
		</div>
	);
}

export function DesktopWelcome() {
	const { t } = useTranslation();
	const [open, setOpen] = useState(() => localStorage.getItem("one_desktop_setup_seen") !== "1");
	function done() {
		localStorage.setItem("one_desktop_setup_seen", "1");
		setOpen(false);
	}
	return (
		<Dialog
			open={open}
			onOpenChange={(value) => {
				if (!value) done();
			}}
		>
			<DialogContent className="max-h-[90dvh] overflow-auto ud-sm:max-w-2xl">
				<DialogHeader>
					<DialogTitle>{t("desktop.welcome")}</DialogTitle>
					<DialogDescription>{t("desktop.welcomeHint")}</DialogDescription>
				</DialogHeader>
				<SettingsBody onDone={done} />
			</DialogContent>
		</Dialog>
	);
}
