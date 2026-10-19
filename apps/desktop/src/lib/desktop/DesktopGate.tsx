import { Fragment, useCallback, useEffect, useState, type ReactNode } from "react";
import { invoke } from "@tauri-apps/api/core";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from "@/components/ui/card";
import { FieldGroup, Field, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Spinner } from "@/components/ui/spinner";
import { http } from "@/lib/http";
import { native, useDesktop, type Connection } from "./runtime";

interface ServerInfo {
	apiVersion: number;
	mode: "local" | "remote";
	id: string;
	generation: string;
	capabilities: Record<string, boolean>;
}

export function DesktopGate({ children }: { children: ReactNode }) {
	const { t } = useTranslation();
	const state = useDesktop();
	const [address, setAddress] = useState(() =>
		native ? (localStorage.getItem("one_server_address") ?? "") : window.location.origin,
	);
	const [token, setToken] = useState("");
	const [busy, setBusy] = useState(true);
	const [mounted, setMounted] = useState(false);
	const [error, setError] = useState("");
	const finish = useCallback(async () => {
		const info = await http.get<ServerInfo>("/server/info");
		if (info.apiVersion !== 1) throw new Error(t("server.incompatible"));
		if (native)
			await invoke("local_operation", {
				operation: "remoteToken",
				payload: { url: address, token, save: true },
			});
		useDesktop.setState({
			connected: true,
			status: "connected",
			mode: info.mode,
			localConnection: false,
			serverURL: address,
			capabilities: info.capabilities,
			generation: `${info.id}:${info.generation}:${crypto.randomUUID()}`,
			error: "",
		});
		setMounted(true);
		setToken("");
	}, [address, token, t]);
	async function local() {
		setBusy(true);
		setError("");
		try {
			await invoke("retry_backend");
			let snapshot: Connection | undefined;
			for (let i = 0; i < 100; i++) {
				snapshot = await invoke<Connection>("get_backend_connection");
				if (snapshot.status === "httpReady") break;
				if (snapshot.status === "failed") throw new Error(snapshot.error);
				await new Promise((resolve) => setTimeout(resolve, 150));
			}
			if (!snapshot || snapshot.status !== "httpReady") throw new Error(t("server.unavailable"));
			http.configure(snapshot.port, snapshot.token ?? "", snapshot.generation);
			const info = await http.get<ServerInfo>("/server/info");
			if (info.apiVersion !== 1) throw new Error(t("server.incompatible"));
			await invoke("confirm_health", { generation: snapshot.generation });
			useDesktop.setState({
				connected: true,
				status: "connected",
				mode: "local",
				localConnection: true,
				serverURL: `http://127.0.0.1:${snapshot.port}`,
				capabilities: info.capabilities,
				generation: `${info.id}:${info.generation}:${crypto.randomUUID()}`,
				error: "",
			});
			localStorage.removeItem("one_server_address");
			setMounted(true);
			setToken("");
		} catch (cause) {
			setError(
				t(`desktop.native.${String((cause as Error).message ?? cause)}`, {
					defaultValue: String((cause as Error).message ?? cause),
				}),
			);
		} finally {
			setBusy(false);
		}
	}
	async function remote() {
		setBusy(true);
		setError("");
		try {
			if (native) {
				http.configureServer(address, token);
			} else {
				const response = await fetch("/api/v1/auth/session", {
					method: "POST",
					headers: { "Content-Type": "application/json" },
					credentials: "same-origin",
					body: JSON.stringify({ token }),
				});
				if (!response.ok) throw new Error(t("server.signInFailed"));
				http.configureServer(window.location.origin, "", "", true);
			}
			await finish();
			if (native) localStorage.setItem("one_server_address", address);
		} catch (cause) {
			http.disconnect();
			setError(
				t(`desktop.native.${String((cause as Error).message ?? cause)}`, {
					defaultValue: String((cause as Error).message ?? cause),
				}),
			);
		} finally {
			setBusy(false);
		}
	}
	useEffect(() => {
		let disposed = false;
		if (native) {
			const saved = localStorage.getItem("one_server_address");
			if (!saved) {
				void local();
				return;
			}
			void invoke<{ token: string }>("local_operation", {
				operation: "remoteToken",
				payload: { url: saved },
			})
				.then(async (credential) => {
					if (disposed || !credential.token) return;
					http.configureServer(saved, credential.token);
					const info = await http.get<ServerInfo>("/server/info");
					if (disposed) return;
					if (info.apiVersion !== 1) throw new Error(t("server.incompatible"));
					useDesktop.setState({
						connected: true,
						status: "connected",
						mode: info.mode,
						localConnection: false,
						serverURL: saved,
						capabilities: info.capabilities,
						generation: `${info.id}:${info.generation}:${crypto.randomUUID()}`,
					});
					setMounted(true);
				})
				.catch((cause) => {
					if (!disposed) {
						http.disconnect();
						setError(
							t(`desktop.native.${String((cause as Error).message ?? cause)}`, {
								defaultValue: String((cause as Error).message ?? cause),
							}),
						);
					}
				})
				.finally(() => {
					if (!disposed) setBusy(false);
				});
			return () => {
				disposed = true;
			};
		}
		http.configureServer(window.location.origin, "", "", true);
		void http
			.get<ServerInfo>("/server/info")
			.then((info) => {
				if (disposed) return;
				if (info.apiVersion !== 1) throw new Error(t("server.incompatible"));
				useDesktop.setState({
					connected: true,
					status: "connected",
					mode: info.mode,
					localConnection: false,
					serverURL: window.location.origin,
					capabilities: info.capabilities,
					generation: `${info.id}:${info.generation}:${crypto.randomUUID()}`,
				});
				setMounted(true);
			})
			.catch(() => {})
			.finally(() => {
				if (!disposed) setBusy(false);
			});
		return () => {
			disposed = true;
		};
	}, []);
	if (state.connected) return <Fragment key={state.generation}>{children}</Fragment>;
	return (
		<div className="grid min-h-dvh place-items-center bg-background p-6">
			<Card className="w-full max-w-lg">
				<CardHeader>
					<CardTitle>{t("server.connectTitle")}</CardTitle>
					<CardDescription>{t("server.connectHint")}</CardDescription>
				</CardHeader>
				<CardContent className="flex flex-col gap-6">
					<form
						className="flex flex-col gap-6"
						onSubmit={(event) => {
							event.preventDefault();
							void remote();
						}}
					>
						<FieldGroup>
							{native ? (
								<Field>
									<FieldLabel htmlFor="one-server-address">{t("server.address")}</FieldLabel>
									<Input
										id="one-server-address"
										type="url"
										required
										value={address}
										onChange={(event) => setAddress(event.target.value)}
										placeholder="https://one.example.com"
										disabled={busy}
									/>
								</Field>
							) : null}
							<Field>
								<FieldLabel htmlFor="one-server-token">{t("server.token")}</FieldLabel>
								<Input
									id="one-server-token"
									type="password"
									autoComplete="current-password"
									required
									value={token}
									onChange={(event) => setToken(event.target.value)}
									disabled={busy}
								/>
							</Field>
						</FieldGroup>
						<Button type="submit" disabled={busy}>
							{busy ? <Spinner /> : null}
							{t("server.connect")}
						</Button>
					</form>
					{native ? (
						<Button variant="outline" disabled={busy} onClick={() => void local()}>
							{t("server.useLocal")}
						</Button>
					) : null}
					{error ? (
						<p role="alert" className="break-words text-sm text-error-foreground">
							{error}
						</p>
					) : null}
					{mounted ? (
						<Button
							variant="ghost"
							disabled={busy}
							onClick={() => {
								http.disconnect();
								window.location.reload();
							}}
						>
							{t("desktop.retry")}
						</Button>
					) : null}
				</CardContent>
			</Card>
		</div>
	);
}
