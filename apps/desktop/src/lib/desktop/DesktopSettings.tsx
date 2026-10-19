import { useState } from "react";
import useSWR from "swr";
import { invoke } from "@tauri-apps/api/core";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import {
	Dialog,
	DialogContent,
	DialogHeader,
	DialogTitle,
	DialogDescription,
} from "@/components/ui/dialog";
import { PageHeader } from "@/components/ui/page-layout";
import { native } from "./runtime";
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
	return native ? (
		<SettingsBody />
	) : (
		<PageHeader title={t("server.settingsTitle")} description={t("server.webHint")} />
	);
}
function SettingsBody({ onDone }: { onDone?: () => void }) {
	const { t } = useTranslation();
	const state = useSWR("/desktop/installation", () =>
		invoke<Installation>("local_operation", { operation: "installation", payload: {} }),
	);
	const [busy, setBusy] = useState(false);
	const [error, setError] = useState("");
	const [update, setUpdate] = useState<Update>();
	const s = state.data;
	async function action(work: () => Promise<unknown>) {
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
		<div className="mx-auto max-w-4xl space-y-6">
			{!onDone ? (
				<PageHeader title={t("desktop.settings")} description={t("desktop.settingsHint")} />
			) : null}
			<Card>
				<CardContent className="space-y-4 p-6">
					<h2 className="text-lg font-semibold">{t("desktop.terminal")}</h2>
					<p className="text-sm text-muted-foreground">{t("desktop.installHint")}</p>
					{s ? (
						<>
							<dl className="grid gap-2 text-sm">
								<dt className="text-muted-foreground">{t("desktop.bundledVersion")}</dt>
								<dd>{s.bundledVersion}</dd>
								<dt className="text-muted-foreground">{t("desktop.target")}</dt>
								<dd className="break-all font-mono">{s.target}</dd>
								<dt className="text-muted-foreground">{t("desktop.existingVersion")}</dt>
								<dd>{s.version || t("desktop.notInstalled")}</dd>
								<dt className="text-muted-foreground">{t("desktop.configuration")}</dt>
								<dd className="break-all font-mono">{s.configPath}</dd>
							</dl>
							<p>{t(`desktop.installAction.${s.action}`)}</p>
							{!s.pathReady ? (
								<div className="rounded-md bg-muted p-3 text-sm">
									<p>{t("desktop.pathPending")}</p>
									<p className="mt-2 break-all">{s.pathHint}</p>
									{s.resolvedPath ? (
										<p className="mt-2 break-all">
											{t("desktop.resolvedPath", { path: s.resolvedPath })}
										</p>
									) : null}
								</div>
							) : (
								<p>{t("desktop.pathDetected")}</p>
							)}
							<div className="flex flex-wrap gap-2">
								{["install", "upgrade"].includes(s.action) ? (
									<Button
										disabled={busy}
										onClick={() =>
											void action(async () => {
												await invoke("local_operation", {
													operation: "installTerminal",
													payload: { target: s.target, digest: s.digest },
												});
												await state.mutate();
											})
										}
									>
										{busy ? t("desktop.working") : t("desktop.install")}
									</Button>
								) : null}
								<Button variant="outline" disabled={busy} onClick={() => void state.mutate()}>
									{t("desktop.refresh")}
								</Button>
							</div>
						</>
					) : (
						<p>{state.error?.message ?? t("desktop.working")}</p>
					)}
				</CardContent>
			</Card>
			{!onDone ? (
				<Card>
					<CardContent className="space-y-4 p-6">
						<h2 className="text-lg font-semibold">{t("desktop.updates")}</h2>
						<p className="text-sm text-muted-foreground">{t("desktop.updateHint")}</p>
						<Button
							disabled={busy}
							variant="outline"
							onClick={() =>
								void action(async () => setUpdate(await invoke<Update>("check_update")))
							}
						>
							{t("desktop.checkUpdate")}
						</Button>
						{update ? (
							<>
								<p>
									{update.enabled
										? update.version
											? t("desktop.updateAvailable", { version: update.version })
											: t("desktop.upToDate")
										: t("desktop.updateUnconfigured")}
								</p>
								{update.notes ? (
									<p className="whitespace-pre-wrap text-sm">{update.notes}</p>
								) : null}
								{update.version ? (
									<Button
										disabled={busy}
										onClick={() => void action(() => invoke("install_update"))}
									>
										{t("desktop.installUpdate")}
									</Button>
								) : null}
							</>
						) : null}
					</CardContent>
				</Card>
			) : null}
			{error ? (
				<p role="alert" className="break-all text-sm text-error-foreground">
					{t(`desktop.native.${error}`, { defaultValue: error })}
				</p>
			) : null}
			{onDone ? (
				<Button variant="outline" disabled={busy} onClick={onDone}>
					{t(s?.installed ? "desktop.continue" : "desktop.skipTerminal")}
				</Button>
			) : null}
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
