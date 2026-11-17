import { useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import useSWR from "swr";
import { SettingsPage, SettingsSection } from "@/components/settings-layout";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ErrorNotice } from "@/components/ui/page-layout";
import { Skeleton } from "@/components/ui/skeleton";
import type { HttpError } from "@/types/api";
import { codeHostsKey, getCodeHosts, saveCodeHost, testCodeHost, type CodeHost } from "./api";

export function CodeHostSettings() {
	const { t } = useTranslation();
	const hosts = useSWR(codeHostsKey, getCodeHosts);
	const [editing, setEditing] = useState<CodeHost>();
	const [savedAccount, setSavedAccount] = useState("");

	async function saved(host: CodeHost) {
		setEditing(undefined);
		setSavedAccount(host.account ?? "");
		await hosts.mutate(
			(current) => [...(current ?? []).filter((item) => item.host !== host.host), host],
			{ revalidate: false },
		);
	}

	const github = hosts.data?.find((host) => host.provider === "github");
	const gitea = hosts.data?.filter((host) => host.provider === "gitea") ?? [];
	return (
		<SettingsPage title={t("codeHosts.settings")}>
			{hosts.error && (
				<SettingsSection title={t("codeHosts.settings")}>
					<ErrorNotice>
						{(hosts.error as HttpError).message || t("codeHosts.loadError")}
					</ErrorNotice>
					<Button variant="outline" className="self-start" onClick={() => void hosts.mutate()}>
						{t("codeHosts.refresh")}
					</Button>
				</SettingsSection>
			)}
			{hosts.isLoading && <Skeleton className="h-24 w-full" aria-busy="true" />}
			{github && (
				<SettingsSection title="GitHub">
					<p className="text-sm text-muted-foreground">{t("codeHosts.githubHint")}</p>
					<ConnectionTest host={github} />
				</SettingsSection>
			)}
			{hosts.data && (
				<SettingsSection title="Gitea">
					{gitea.length > 0 && (
						<ul className="divide-y divide-border">
							{gitea.map((host) => (
								<li key={host.host} className="grid gap-2 py-3 first:pt-0">
									<div className="flex flex-wrap items-center justify-between gap-2">
										<div className="min-w-0">
											<p className="break-all text-sm font-medium">{host.url}</p>
											{host.account && (
												<p className="text-xs text-muted-foreground">{host.account}</p>
											)}
										</div>
										<Button
											type="button"
											variant="outline"
											onClick={() => {
												setEditing(host);
												setSavedAccount("");
											}}
										>
											{t("codeHosts.updateToken")}
										</Button>
									</div>
									<ConnectionTest key={host.account} host={host} />
								</li>
							))}
						</ul>
					)}
					<GiteaForm
						key={editing?.host ?? "new"}
						editing={editing}
						onSaved={saved}
						onCancel={() => setEditing(undefined)}
					/>
					{savedAccount && (
						<p role="status" className="text-sm text-muted-foreground">
							{t("codeHosts.saved", { account: savedAccount })}
						</p>
					)}
				</SettingsSection>
			)}
		</SettingsPage>
	);
}

function ConnectionTest({ host }: { host: CodeHost }) {
	const { t } = useTranslation();
	const [pending, setPending] = useState(false);
	const [error, setError] = useState("");
	const [account, setAccount] = useState("");

	async function test() {
		setPending(true);
		setAccount("");
		setError("");
		try {
			const result = await testCodeHost(host.host);
			setAccount(result.account ?? "");
		} catch (e) {
			setError((e as HttpError).message || t("codeHosts.loadError"));
		} finally {
			setPending(false);
		}
	}

	return (
		<div className="grid gap-2">
			<div className="flex flex-wrap items-center gap-2">
				<Button variant="outline" type="button" disabled={pending} onClick={() => void test()}>
					{t(pending ? "codeHosts.testing" : "codeHosts.test")}
				</Button>
				{account && (
					<p role="status" className="text-sm text-muted-foreground">
						{t("codeHosts.connected", { account })}
					</p>
				)}
			</div>
			{error && <ErrorNotice>{error}</ErrorNotice>}
		</div>
	);
}

function GiteaForm({
	editing,
	onSaved,
	onCancel,
}: {
	editing?: CodeHost;
	onSaved(host: CodeHost): Promise<void>;
	onCancel(): void;
}) {
	const { t } = useTranslation();
	const [url, setURL] = useState(editing?.url ?? "");
	const [token, setToken] = useState("");
	const [pending, setPending] = useState(false);
	const [error, setError] = useState("");
	const canSave = !!url.trim() && !!token.trim() && !pending;

	async function save(event: FormEvent<HTMLFormElement>) {
		event.preventDefault();
		if (!canSave) return;
		setPending(true);
		setError("");
		try {
			const host = await saveCodeHost(url, token);
			setToken("");
			setURL("");
			await onSaved(host);
		} catch (e) {
			setError((e as HttpError).message || t("codeHosts.loadError"));
		} finally {
			setPending(false);
		}
	}

	return (
		<form className="grid max-w-lg gap-3" onSubmit={(event) => void save(event)}>
			<h3 className="text-sm font-medium">
				{t(editing ? "codeHosts.updateToken" : "codeHosts.addGitea")}
			</h3>
			<div className="grid gap-2">
				<Label htmlFor="gitea-url">{t("codeHosts.address")}</Label>
				<Input
					id="gitea-url"
					type="url"
					value={url}
					autoComplete="off"
					required
					readOnly={!!editing}
					disabled={pending}
					placeholder="https://git.example.com"
					onChange={(event) => setURL(event.target.value)}
				/>
			</div>
			<div className="grid gap-2">
				<Label htmlFor="gitea-token">{t("codeHosts.token")}</Label>
				<Input
					id="gitea-token"
					type="password"
					autoComplete="new-password"
					value={token}
					required
					disabled={pending}
					aria-describedby="gitea-token-hint"
					onChange={(event) => setToken(event.target.value)}
				/>
				<p id="gitea-token-hint" className="text-xs text-muted-foreground">
					{t("codeHosts.tokenHint")}
				</p>
			</div>
			<div className="flex flex-wrap gap-2">
				<Button type="submit" disabled={!canSave}>
					{t(pending ? "codeHosts.saving" : "codeHosts.verifyAndSave")}
				</Button>
				{editing && (
					<Button variant="outline" type="button" onClick={onCancel} disabled={pending}>
						{t("form.cancel")}
					</Button>
				)}
			</div>
			{error && <ErrorNotice>{error}</ErrorNotice>}
		</form>
	);
}
