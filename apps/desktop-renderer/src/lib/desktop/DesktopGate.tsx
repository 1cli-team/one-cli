import { useEffect, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { PageHeader } from "@/components/ui/page-layout";
import { Spinner } from "@/components/ui/spinner";
import { http } from "@/lib/http";
import i18n from "@/lib/i18n";
import { useLocaleStore } from "@/lib/stores/locale";
import { desktop, native, useDesktop } from "./runtime";

export function DesktopGate({ children }: { children: ReactNode }) {
	const { t } = useTranslation();
	const locale = useLocaleStore((s) => s.resolved);
	const state = useDesktop();
	const [mounted, setMounted] = useState(false);
	const [attempt, setAttempt] = useState(0);
	useEffect(() => {
		if (i18n.language !== locale) void i18n.changeLanguage(locale);
	}, [locale]);
	useEffect(() => {
		if (!native) {
			useDesktop.setState({ status: "preview", error: "", connected: false });
			return;
		}
		let disposed = false;
		let newest = -1;
		let probing = "";
		let unlisten: (() => void) | undefined;
		async function refresh() {
			const snapshot = await desktop().getBackendConnection();
			if (disposed || snapshot.sequence < newest) return;
			newest = snapshot.sequence;
			if (snapshot.status !== "httpReady") {
				http.disconnect();
				probing = "";
				useDesktop.setState({ status: snapshot.status, error: snapshot.error, connected: false });
				return;
			}
			if (probing === snapshot.generation) return;
			probing = snapshot.generation;
			http.configure(snapshot.port, snapshot.token ?? "", snapshot.generation);
			useDesktop.setState({ status: "connecting", connected: false, error: "" });
			try {
				const result = await http.get<{
					generation: string;
					identity: { api: number; version: string; commit: string };
				}>("/desktop/session");
				if (disposed || newest !== snapshot.sequence) return;
				if (result.generation !== snapshot.generation || result.identity.api !== 1)
					throw new Error(i18n.t("desktop.connectionInvalid"));
				await desktop().confirmHealth(snapshot.generation);
				useDesktop.setState({
					status: "connected",
					connected: true,
					generation: snapshot.generation,
					error: "",
				});
				setMounted(true);
			} catch (error) {
				if (!disposed && newest === snapshot.sequence) {
					probing = "";
					useDesktop.setState({
						status: "connectionFailed",
						connected: false,
						error: (error as Error).message,
					});
				}
			}
		}
		void (async () => {
			unlisten = desktop().onBackendState(() => {
				void refresh().catch(fail);
			});
			if (disposed) {
				unlisten();
				return;
			}
			await refresh();
		})().catch(fail);
		function fail(error: unknown) {
			if (!disposed)
				useDesktop.setState({ status: "failed", error: String(error), connected: false });
		}
		return () => {
			disposed = true;
			unlisten?.();
		};
	}, [attempt]);
	const canRetry = native && ["failed", "connectionFailed", "stopped"].includes(state.status);
	const showRecovery = canRetry || state.status === "preview";
	const opening = state.status === "starting" || state.status === "connecting";
	const status = opening ? (mounted ? "reconnecting" : "starting") : state.status;
	const message = t(`desktop.connection.${status}`, {
		defaultValue: t("desktop.connection.starting"),
	});
	const errorKey = `desktop.native.${state.error}`;
	const knownError = state.error && i18n.exists(errorKey);
	return (
		<>
			{mounted ? (
				<div inert={!state.connected} className="contents">
					{children}
				</div>
			) : null}
			{!state.connected ? (
				<div className="window-fallback fixed inset-0 z-50 grid place-items-center bg-background/95 p-6 text-foreground">
					{showRecovery ? (
						<div className="flex w-full max-w-md flex-col gap-4 rounded-lg border border-border bg-card p-6">
							<PageHeader
								title="One Desktop"
								description={<span role={canRetry ? "alert" : undefined}>{message}</span>}
							/>
							{knownError ? (
								<p className="text-sm text-muted-foreground">{t(errorKey)}</p>
							) : state.error ? (
								<details className="text-sm text-muted-foreground">
									<summary className="w-fit cursor-pointer rounded-sm focus-visible:outline-2 focus-visible:outline-ring">
										{t("desktop.connection.details")}
									</summary>
									<p className="mt-2 wrap-anywhere whitespace-pre-wrap">{state.error}</p>
								</details>
							) : null}
							{canRetry && (
								<Button
									className="self-start"
									onClick={() => {
										void (
											state.status === "connectionFailed"
												? Promise.resolve()
												: desktop().retryBackend()
										).then(() => setAttempt((n) => n + 1));
									}}
								>
									{t("desktop.retry")}
								</Button>
							)}
						</div>
					) : (
						<div role="status" className="flex max-w-sm flex-col items-center gap-3 text-center">
							<Spinner
								aria-hidden="true"
								className="size-6 text-primary motion-reduce:animate-none"
							/>
							<p className="text-sm font-medium">{message}</p>
						</div>
					)}
				</div>
			) : null}
		</>
	);
}
