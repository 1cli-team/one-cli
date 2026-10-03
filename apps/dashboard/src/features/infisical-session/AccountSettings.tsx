import {
	ExternalLink,
	Languages,
	LogIn,
	LogOut,
	MoonStar,
	SunMedium,
	RefreshCw,
	Settings2,
	ShieldCheck,
} from "lucide-react";
import { SharedCredentialsNotice } from "./SharedCredentialsNotice";
import { Badge } from "@/components/ui/badge";
import { ErrorNotice, PageHeader, SectionHeading } from "@/components/ui/page-layout";
import { Skeleton } from "@/components/ui/skeleton";
import { Spinner } from "@/components/ui/spinner";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import useSWR, { useSWRConfig } from "swr";
import { cancelLogin, getSession, logout, message, sessionKey, startLogin } from "@/api/session";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
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

export function AccountSettings() {
	const { t } = useTranslation();
	const { mode: theme, setMode: setTheme } = useThemeStore();
	const state = useSWR(sessionKey, getSession, { refreshInterval: 2000 });
	const { mutate } = useSWRConfig();
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
		// Open synchronously from the click to avoid popup blocking after the API call.
		const popup = window.open("about:blank", "_blank");
		if (popup) popup.opener = null;
		await perform(async () => {
			try {
				const attempt = await startLogin(site);
				if (popup) popup.location.href = attempt.url;
			} catch (e) {
				popup?.close();
				throw e;
			}
		});
	}
	return (
		<div className="mx-auto w-full max-w-5xl space-y-6">
			<PageHeader title={t("topbar.settings")} description={t("session.settingsHint")} />
			<Card>
				<CardContent className="space-y-6 p-6">
					<SectionHeading
						icon={ShieldCheck}
						title="Infisical"
						description={t("session.description")}
						actions={
							state.data?.session.loggedIn ? (
								<Badge variant="success">{t("session.connected")}</Badge>
							) : undefined
						}
					/>
					{state.isLoading && !state.data ? (
						<div role="status" aria-label={t("session.loading")} className="space-y-3">
							<Skeleton className="h-12" />
							<Skeleton className="h-12 w-2/3" />
						</div>
					) : state.data?.session.loggedIn ? (
						<>
							<div className="grid gap-6 rounded-lg bg-muted/40 p-4 ud-sm:grid-cols-2">
								<div className="space-y-1">
									<p className="text-xs text-muted-foreground">{t("session.account")}</p>
									<p className="break-all font-medium">{state.data.session.email || "—"}</p>
								</div>
								<div className="space-y-1">
									<p className="text-xs text-muted-foreground">{t("session.site")}</p>
									<p className="break-all text-sm">{state.data.session.siteUrl}</p>
								</div>
								<div className="space-y-1 ud-sm:col-span-2">
									<p className="text-xs text-muted-foreground">{t("session.organization")}</p>
									<p className="break-all font-mono text-xs">
										{state.data.session.organizationId || "—"}
									</p>
								</div>
							</div>
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
							<div className="flex flex-wrap items-center justify-end gap-3">
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
														(key.includes("secrets") ||
															key.includes("infisical") ||
															key === "/global-env/location")) ||
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
										<Button asChild>
											<a href={state.data?.login?.url} target="_blank" rel="noreferrer">
												<ExternalLink />
												{t("session.reopen")}
											</a>
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
				</CardContent>
			</Card>
			<Card>
				<CardContent className="flex flex-wrap items-center justify-between gap-4 p-6">
					<SectionHeading
						icon={Languages}
						title={t("session.language")}
						description={t("session.languageHint")}
					/>
					<LanguageSwitcher showLabel />
				</CardContent>
			</Card>
			<Card>
				<CardContent className="flex flex-wrap items-center justify-between gap-4 p-6">
					<SectionHeading
						icon={MoonStar}
						title={t("session.theme")}
						description={t("session.themeHint")}
					/>
					<Select value={theme} onValueChange={(value) => setTheme(value as ThemeMode)}>
						<SelectTrigger className="w-40" aria-label={t("session.theme")}>
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
				</CardContent>
			</Card>
		</div>
	);
}

export function SessionStatus() {
	const { t } = useTranslation();
	const state = useSWR(sessionKey, getSession, { refreshInterval: 5000 });
	return (
		<span
			className="block truncate text-xs text-muted-foreground"
			title={state.data?.session.email}
		>
			{state.error
				? t("session.unavailable")
				: state.data?.session.loggedIn
					? state.data.session.email
					: t("session.signedOut")}
		</span>
	);
}
