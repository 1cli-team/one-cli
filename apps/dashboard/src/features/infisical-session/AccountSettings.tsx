import { useState } from "react";
import { useTranslation } from "react-i18next";
import useSWR, { useSWRConfig } from "swr";
import { cancelLogin, getSession, logout, message, sessionKey, startLogin } from "@/api/session";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { LanguageSwitcher } from "@/components/LanguageSwitcher";

export function AccountSettings() {
	const { t } = useTranslation();
	const state = useSWR(sessionKey, getSession, { refreshInterval: 2000 });
	const { mutate } = useSWRConfig();
	const [site, setSite] = useState("https://app.infisical.com");
	const [custom, setCustom] = useState(false);
	const [busy, setBusy] = useState(false);
	const [error, setError] = useState("");
	const waiting = state.data?.login?.status === "waiting";
	async function perform(action: () => Promise<unknown>) {
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
		<div className="mx-auto w-full max-w-3xl space-y-4">
			<Card>
				<CardContent className="space-y-4 p-5">
					<div>
						<h2 className="text-lg font-semibold">Infisical</h2>
						<p className="mt-1 text-sm text-muted-foreground">{t("session.description")}</p>
					</div>
					{state.isLoading ? <p role="status">{t("session.loading")}</p> : null}
					{state.data?.session.loggedIn ? (
						<>
							<p className="font-medium">{state.data.session.email}</p>
							<p className="break-all font-mono text-xs text-muted-foreground">
								{state.data.session.siteUrl}
							</p>
							<p className="text-xs text-muted-foreground">
								{t("session.organization")}: {state.data.session.organizationId || "—"}
							</p>
							<Button
								variant="outline"
								disabled={busy}
								onClick={() =>
									void perform(async () => {
										await logout();
										await mutate(
											(key) =>
												typeof key === "string" &&
												(key.includes("secrets") || key.includes("infisical")),
											undefined,
											{ revalidate: false },
										);
									})
								}
							>
								{t("session.logout")}
							</Button>
						</>
					) : (
						<>
							<p>{state.data?.session.expired ? t("session.expired") : t("session.signedOut")}</p>
							{waiting ? (
								<div className="space-y-3">
									<p role="status">{t("session.waiting")}</p>
									<div className="flex flex-wrap gap-2">
										<Button asChild>
											<a href={state.data?.login?.url} target="_blank" rel="noreferrer">
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
								<>
									<Button disabled={busy || state.isLoading} onClick={() => void login()}>
										{t("session.login")}
									</Button>
									<Button
										variant="ghost"
										onClick={() => {
											setCustom(!custom);
											if (custom) setSite("https://app.infisical.com");
										}}
									>
										{t("session.custom")}
									</Button>
									{custom ? (
										<div className="space-y-2">
											<Label htmlFor="infisical-site">{t("session.site")}</Label>
											<Input
												id="infisical-site"
												value={site}
												onChange={(e) => setSite(e.target.value)}
												placeholder="https://app.infisical.com"
											/>
										</div>
									) : null}
								</>
							)}
						</>
					)}
					{error || state.error || state.data?.error ? (
						<p role="alert" className="text-sm text-error-foreground">
							{error || (state.error ? message(state.error) : state.data?.error)}
						</p>
					) : null}
				</CardContent>
			</Card>
			<Card>
				<CardContent className="flex items-center justify-between p-5">
					<Label>{t("session.language")}</Label>
					<LanguageSwitcher />
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
