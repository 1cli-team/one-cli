import { useEffect, useState, type ReactNode } from "react";
import { invoke } from "@tauri-apps/api/core";
import { listen } from "@tauri-apps/api/event";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { http } from "@/lib/http";
import i18n from "@/lib/i18n";
import { useLocaleStore } from "@/lib/stores/locale";
import { native, useDesktop, type Connection } from "./runtime";

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
			const snapshot = await invoke<Connection>("get_backend_connection");
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
				await invoke("confirm_health", { generation: snapshot.generation });
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
			unlisten = await listen("backend-state", () => {
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
	return (
		<>
			{mounted ? (
				<div inert={!state.connected} className="contents">
					{children}
				</div>
			) : null}
			{!state.connected ? (
				<div
					role="status"
					className="fixed inset-0 z-50 grid place-items-center bg-background/95 p-6 text-foreground"
				>
					<div className="w-full max-w-lg space-y-4 rounded-xl border bg-card p-8 shadow-sm">
						<h1 className="text-2xl font-semibold">One Desktop</h1>
						<p>
							{t(`desktop.connection.${state.status}`, {
								defaultValue: t("desktop.connection.starting"),
							})}
						</p>
						{state.error ? (
							<p className="break-all text-sm text-muted-foreground">
								{t(`desktop.native.${state.error}`, { defaultValue: state.error })}
							</p>
						) : null}
						{native && ["failed", "connectionFailed", "stopped"].includes(state.status) ? (
							<Button
								onClick={() => {
									void (
										state.status === "connectionFailed"
											? Promise.resolve()
											: invoke("retry_backend")
									).then(() => setAttempt((n) => n + 1));
								}}
							>
								{t("desktop.retry")}
							</Button>
						) : null}
					</div>
				</div>
			) : null}
		</>
	);
}
