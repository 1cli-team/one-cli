import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import useSWR from "swr";
import { ExternalLink, Link2, Unplug } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ErrorNotice } from "@/components/ui/page-layout";
import { SettingsPage, SettingsSection } from "@/components/settings-layout";
import { openExternal } from "@/lib/desktop/runtime";
import {
	authorize,
	cancelAuthorization,
	connectionKey,
	disconnect,
	errorText,
	fetchJSON,
	saveConfig,
	type ConnectionState,
	type OAuthConfig,
} from "./api";

export function LinearSettings() {
	const { t } = useTranslation();
	const state = useSWR<ConnectionState>(connectionKey, fetchJSON, {
		refreshInterval: (data) => (data?.authorization?.status === "pending" ? 1500 : 0),
	});
	const [config, setConfig] = useState<OAuthConfig>({
		clientId: "",
		redirectUri: "http://127.0.0.1:39127/oauth/linear/callback",
	});
	const [dirty, setDirty] = useState(false);
	const [busy, setBusy] = useState(false);
	const [error, setError] = useState("");
	useEffect(() => {
		if (state.data && !dirty) setConfig(state.data.config);
	}, [state.data, dirty]);
	async function act(work: () => Promise<unknown>) {
		setBusy(true);
		setError("");
		try {
			await work();
			await state.mutate();
		} catch (e) {
			setError(errorText(e));
		} finally {
			setBusy(false);
		}
	}
	const pending = state.data?.authorization?.status === "pending";
	return (
		<SettingsPage title={t("linear.settings")}>
			<SettingsSection title={t("linear.settings")}>
				<p className="text-sm text-muted-foreground">{t("linear.oauthSetup")}</p>
				<Button
					variant="outline"
					className="self-start"
					onClick={() =>
						void act(() => openExternal("https://linear.app/settings/api/applications/new"))
					}
				>
					<ExternalLink />
					{t("linear.registerApp")}
				</Button>
				<form
					className="grid gap-4"
					onSubmit={(e) => {
						e.preventDefault();
						void act(async () => {
							await saveConfig(config);
							setDirty(false);
						});
					}}
				>
					<div className="space-y-2">
						<Label htmlFor="linear-client">Client ID</Label>
						<Input
							id="linear-client"
							required
							value={config.clientId}
							disabled={busy || pending}
							onChange={(e) => {
								setDirty(true);
								setConfig({ ...config, clientId: e.target.value });
							}}
						/>
					</div>
					<div className="space-y-2">
						<Label htmlFor="linear-redirect">Redirect URI</Label>
						<Input
							id="linear-redirect"
							required
							value={config.redirectUri}
							disabled={busy || pending}
							onChange={(e) => {
								setDirty(true);
								setConfig({ ...config, redirectUri: e.target.value });
							}}
						/>
						<p className="text-xs text-muted-foreground">{t("linear.redirectHint")}</p>
					</div>
					<Button
						className="justify-self-start"
						type="submit"
						disabled={busy || pending || !config.clientId.trim()}
					>
						{t("linear.saveConfig")}
					</Button>
				</form>
				<div className="flex flex-wrap gap-2">
					<Button
						disabled={busy || pending || !state.data?.config.clientId || dirty}
						onClick={() =>
							void act(async () => {
								const auth = await authorize();
								await state.mutate();
								await openExternal(auth.url);
							})
						}
					>
						<Link2 />
						{t("linear.connect")}
					</Button>
					{pending && (
						<>
							<Button
								variant="outline"
								disabled={busy}
								onClick={() => void act(() => openExternal(state.data!.authorization!.url))}
							>
								{t("linear.reopen")}
							</Button>
							<Button variant="ghost" disabled={busy} onClick={() => void act(cancelAuthorization)}>
								{t("git.cancel")}
							</Button>
						</>
					)}
				</div>
				{state.data?.authorization && (
					<p role="status" className="text-sm text-muted-foreground">
						{t(`linear.auth.${state.data.authorization.status}`)}
					</p>
				)}
				{(error || state.error) && <ErrorNotice>{error || errorText(state.error)}</ErrorNotice>}
			</SettingsSection>
			<SettingsSection title={t("linear.accounts")}>
				{state.data?.connections.map((connection) => (
					<div
						key={connection.id}
						className="flex items-center justify-between gap-4 border-b border-border py-2 last:border-0"
					>
						<div>
							<p className="text-sm font-medium">{connection.name}</p>
							<p className="text-xs text-muted-foreground">{connection.organizationName}</p>
						</div>
						<Button
							variant="outline"
							disabled={busy}
							onClick={() => void act(() => disconnect(connection.id))}
						>
							<Unplug />
							{t("linear.disconnect")}
						</Button>
					</div>
				))}
				{!state.data?.connections.length && (
					<p className="text-sm text-muted-foreground">{t("linear.noAccounts")}</p>
				)}
			</SettingsSection>
		</SettingsPage>
	);
}
