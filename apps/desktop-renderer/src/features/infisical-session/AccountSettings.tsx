import { openExternal } from "@/lib/desktop/runtime";
import {
	ExternalLink,
	LogIn,
	LogOut,
	MoonStar,
	SunMedium,
	RefreshCw,
	Settings2,
} from "lucide-react";
import { SharedCredentialsNotice } from "./SharedCredentialsNotice";
import { Badge } from "@/components/ui/badge";
import { ErrorNotice } from "@/components/ui/page-layout";
import {
	SettingsDetails,
	SettingsPage,
	SettingsRow,
	SettingsSection,
} from "@/components/settings-layout";
import { Skeleton } from "@/components/ui/skeleton";
import { Spinner } from "@/components/ui/spinner";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import useSWR from "swr";
import { useSessionSWRConfig } from "@/lib/desktop/cache";
import { cancelLogin, getSession, logout, message, sessionKey, startLogin } from "@/api/session";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { LanguageSwitcher } from "@/components/LanguageSwitcher";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import { useThemeStore, type ThemeMode } from "@/lib/stores/theme";
import { GatewayDownloadSettings } from "@/features/gateway/GatewayDownloadSettings";

export function AccountSettings() {
	const { t } = useTranslation();
	const { mode: theme, setMode: setTheme } = useThemeStore();
	const state = useSWR(sessionKey, getSession, { refreshInterval: 2000 });
	const { mutate } = useSessionSWRConfig();
	const [site, setSite] = useState("https://app.infisical.com");
	const [custom, setCustom] = useState(false);
	const [busy, setBusy] = useState(false);
	const [error, setError] = useState("");
	const waiting = state.data?.login?.status === "waiting";
	async function perform(action: () => Promise<unknown>) {
		if (busy) return;
		setBusy(true);
		setError("");
		try {
			await action();
			await mutate(sessionKey);
		} catch (e) {
			setError(message(e));
		} finally {
			setBusy(false);
		}
	}
	async function login() {
		if (busy) return;
		await perform(async () => {
			const attempt = await startLogin(site);
			await openExternal(attempt.url);
		});
	}
	return (
		<SettingsPage title={t("navigation.generalSettings")}>
			<SettingsSection title={t("session.accountConnection")}>
				{state.isLoading && !state.data ? (
					<div role="status" aria-label={t("session.loading")} className="space-y-3">
						<Skeleton className="h-12" />
						<Skeleton className="h-12 w-2/3" />
					</div>
				) : state.data?.session.loggedIn ? (
					<>
						<SettingsDetails>
							<dt>{t("session.account")}</dt>
							<dd>{state.data.session.email || "—"}</dd>
							<dt>{t("session.site")}</dt>
							<dd className="font-mono">{state.data.session.siteUrl}</dd>
							<dt>{t("session.organization")}</dt>
							<dd className="font-mono">{state.data.session.organizationId || "—"}</dd>
						</SettingsDetails>
						<SharedCredentialsNotice
							key={JSON.stringify([
								state.data.session.siteUrl,
								state.data.session.userId,
								state.data.session.organizationId,
								state.data.session.expiresAt,
							])}
							state={state.data.sharedCredentials}
							account={state.data.session}
						/>
						<div className="flex flex-wrap items-center justify-between gap-2 border-t border-border pt-4">
							<Badge variant="success">{t("session.connected")}</Badge>
							<Button
								variant="ghost"
								disabled={busy}
								onClick={() =>
									void perform(async () => {
										await logout();
										await mutate(
											sessionKey,
											{ session: { loggedIn: false, expired: false } },
											{ revalidate: false },
										);
										await mutate(
											(key) =>
												(typeof key === "string" &&
													(key.includes("secrets") || key.includes("infisical"))) ||
												(Array.isArray(key) && key[0] === "/infisical/projects"),
											undefined,
											{ revalidate: false },
										);
									})
								}
							>
								{busy ? <Spinner /> : <LogOut />}
								{t("session.logout")}
							</Button>
						</div>
					</>
				) : !state.error ? (
					<>
						<p className="text-sm text-muted-foreground">
							{state.data?.session.expired ? t("session.expired") : t("session.signedOut")}
						</p>
						{waiting ? (
							<div className="space-y-4 rounded-lg border border-border bg-muted/30 p-4">
								<p role="status" className="flex items-center gap-2">
									<Spinner />
									{t("session.waiting")}
								</p>
								<div className="flex flex-wrap gap-2">
									<Button
										onClick={() =>
											state.data?.login?.url && void openExternal(state.data.login.url)
										}
									>
										<ExternalLink />
										{t("session.reopen")}
									</Button>

									<Button
										variant="outline"
										disabled={busy}
										onClick={() => void perform(cancelLogin)}
									>
										{t("session.cancel")}
									</Button>
								</div>
							</div>
						) : (
							<form
								className="space-y-5"
								onSubmit={(e) => {
									e.preventDefault();
									if (!busy) void login();
								}}
							>
								{custom && (
									<div className="max-w-xl space-y-2">
										<Label htmlFor="infisical-site">{t("session.site")}</Label>
										<Input
											id="infisical-site"
											type="url"
											required
											value={site}
											onChange={(e) => setSite(e.target.value)}
											placeholder="https://app.infisical.com"
											disabled={busy}
										/>
									</div>
								)}
								<div className="flex flex-wrap gap-2">
									<Button type="submit" disabled={busy}>
										{busy ? <Spinner /> : <LogIn />}
										{t("session.login")}
									</Button>
									<Button
										variant="ghost"
										aria-expanded={custom}
										disabled={busy}
										onClick={() => {
											setCustom(!custom);
											if (custom) setSite("https://app.infisical.com");
										}}
									>
										<Settings2 />
										{t("session.custom")}
									</Button>
								</div>
							</form>
						)}
					</>
				) : null}
				{error || state.error || state.data?.error ? (
					<ErrorNotice
						action={
							state.error ? (
								<Button
									variant="outline"
									size="sm"
									disabled={state.isValidating}
									onClick={() => void state.mutate()}
								>
									<RefreshCw />
									{t("secrets.retry")}
								</Button>
							) : undefined
						}
					>
						{error || (state.error ? message(state.error) : state.data?.error)}
					</ErrorNotice>
				) : null}
			</SettingsSection>
			<SettingsSection title={t("session.preferences")}>
				<div>
					<SettingsRow title={t("session.language")}>
						<LanguageSwitcher showLabel />
					</SettingsRow>
					<SettingsRow title={t("session.theme")}>
						<Select value={theme} onValueChange={(value) => setTheme(value as ThemeMode)}>
							<SelectTrigger className="w-full" aria-label={t("session.theme")}>
								<SelectValue />
							</SelectTrigger>
							<SelectContent>
								<SelectItem value="light">
									<span className="flex items-center gap-2">
										<SunMedium aria-hidden="true" />
										{t("session.themeLight")}
									</span>
								</SelectItem>
								<SelectItem value="dark">
									<span className="flex items-center gap-2">
										<MoonStar aria-hidden="true" />
										{t("session.themeDark")}
									</span>
								</SelectItem>
							</SelectContent>
						</Select>
					</SettingsRow>
				</div>
			</SettingsSection>
			<GatewayDownloadSettings />
		</SettingsPage>
	);
}
