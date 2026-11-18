import { PlusIcon } from "lucide-react";
import { useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import useSWR from "swr";
import { SettingsPage, SettingsSection } from "@/components/settings-layout";
import { Button } from "@/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import { Field, FieldDescription, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { ErrorNotice } from "@/components/ui/page-layout";
import { Skeleton } from "@/components/ui/skeleton";
import type { HttpError } from "@/types/api";
import { codeHostsKey, getCodeHosts, saveCodeHost, testCodeHost, type CodeHost } from "./api";

export function CodeHostSettings() {
	const { t } = useTranslation();
	const hosts = useSWR(codeHostsKey, getCodeHosts);
	const [editor, setEditor] = useState<CodeHost | "add">();

	async function saved(host: CodeHost) {
		await hosts.mutate(
			(current) => [...(current ?? []).filter((item) => item.host !== host.host), host],
			{ revalidate: false },
		);
		setEditor(undefined);
		toast.success(
			host.repository
				? t("codeHosts.repositorySaved", { repository: host.repository })
				: t("codeHosts.saved", { account: host.account }),
		);
	}

	const github = hosts.data?.find((host) => host.provider === "github");
	const gitea = hosts.data?.filter((host) => host.provider === "gitea") ?? [];
	return (
		<SettingsPage title={t("codeHosts.settings")}>
			{hosts.error && (
				<ErrorNotice
					action={
						<Button variant="outline" onClick={() => void hosts.mutate()}>
							{t("codeHosts.refresh")}
						</Button>
					}
				>
					{(hosts.error as HttpError).message || t("codeHosts.loadError")}
				</ErrorNotice>
			)}
			{hosts.isLoading && <Skeleton className="h-24 w-full" aria-busy="true" />}
			{github && (
				<SettingsSection title="GitHub">
					<p className="text-sm text-muted-foreground">{t("codeHosts.githubHint")}</p>
					<ConnectionTest host={github} />
				</SettingsSection>
			)}
			{hosts.data && (
				<SettingsSection title="Gitea / Forgejo">
					{gitea.length > 0 && (
						<ul className="divide-y divide-border">
							{gitea.map((host) => (
								<li key={host.host} className="grid gap-2 py-3 first:pt-0">
									<div className="flex flex-wrap items-center justify-between gap-2">
										<div className="min-w-0">
											<p className="break-all text-sm font-medium">{host.url}</p>
											{(host.repository || host.account) && (
												<p className="break-all text-xs text-muted-foreground">
													{host.repository ?? host.account}
												</p>
											)}
										</div>
										<Button
											type="button"
											variant="outline"
											aria-haspopup="dialog"
											onClick={() => setEditor(host)}
										>
											{t("codeHosts.updateToken")}
										</Button>
									</div>
									<ConnectionTest key={host.repository ?? host.account} host={host} />
								</li>
							))}
						</ul>
					)}
					<Button className="self-start" aria-haspopup="dialog" onClick={() => setEditor("add")}>
						<PlusIcon data-icon="inline-start" aria-hidden="true" />
						{t("codeHosts.addServer")}
					</Button>
				</SettingsSection>
			)}
			{editor && (
				<GiteaDialog
					editing={editor === "add" ? undefined : editor}
					onSaved={saved}
					onClose={() => setEditor(undefined)}
				/>
			)}
		</SettingsPage>
	);
}

function ConnectionTest({ host }: { host: CodeHost }) {
	const { t } = useTranslation();
	const [pending, setPending] = useState(false);
	const [error, setError] = useState("");
	const [verified, setVerified] = useState<CodeHost>();

	async function test() {
		setPending(true);
		setVerified(undefined);
		setError("");
		try {
			setVerified(await testCodeHost(host.host));
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
				{verified && (
					<p role="status" className="text-sm wrap-anywhere text-muted-foreground">
						{verified.repository
							? t("codeHosts.repositoryConnected", { repository: verified.repository })
							: t("codeHosts.connected", { account: verified.account })}
					</p>
				)}
			</div>
			{error && <ErrorNotice>{error}</ErrorNotice>}
		</div>
	);
}

function GiteaDialog({
	editing,
	onSaved,
	onClose,
}: {
	editing?: CodeHost;
	onSaved(host: CodeHost): Promise<void>;
	onClose(): void;
}) {
	const { t } = useTranslation();
	const [url, setURL] = useState(editing?.url ?? "");
	const [repository, setRepository] = useState(editing?.repository ?? "");
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
			const host = await saveCodeHost(url, token, repository);
			await onSaved(host);
		} catch (e) {
			setError((e as HttpError).message || t("codeHosts.loadError"));
		} finally {
			setPending(false);
		}
	}

	return (
		<Dialog open onOpenChange={(open) => !open && !pending && onClose()}>
			<DialogContent showCloseButton={!pending} aria-describedby={undefined}>
				<DialogHeader>
					<DialogTitle>{t(editing ? "codeHosts.updateToken" : "codeHosts.addGitea")}</DialogTitle>
				</DialogHeader>
				<form className="grid gap-4" onSubmit={(event) => void save(event)}>
					<FieldGroup className="gap-4">
						<Field>
							<FieldLabel htmlFor="gitea-url">{t("codeHosts.address")}</FieldLabel>
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
						</Field>
						<Field>
							<FieldLabel htmlFor="gitea-repository">{t("codeHosts.repository")}</FieldLabel>
							<Input
								id="gitea-repository"
								value={repository}
								placeholder="owner/repo"
								autoComplete="off"
								disabled={pending}
								aria-describedby="gitea-repository-hint"
								onChange={(event) => setRepository(event.target.value)}
							/>
							<FieldDescription id="gitea-repository-hint">
								{t("codeHosts.repositoryHint")}
							</FieldDescription>
						</Field>
						<Field>
							<FieldLabel htmlFor="gitea-token">{t("codeHosts.token")}</FieldLabel>
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
							<FieldDescription id="gitea-token-hint">
								{t(repository.trim() ? "codeHosts.repositoryTokenHint" : "codeHosts.tokenHint")}
							</FieldDescription>
						</Field>
					</FieldGroup>
					{error && <ErrorNotice>{error}</ErrorNotice>}
					<DialogFooter>
						<Button variant="outline" type="button" onClick={onClose} disabled={pending}>
							{t("form.cancel")}
						</Button>
						<Button type="submit" disabled={!canSave}>
							{t(pending ? "codeHosts.saving" : "codeHosts.verifyAndSave")}
						</Button>
					</DialogFooter>
				</form>
			</DialogContent>
		</Dialog>
	);
}
