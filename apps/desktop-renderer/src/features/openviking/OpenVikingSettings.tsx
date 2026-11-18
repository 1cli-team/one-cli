import { useState } from "react";
import { useTranslation } from "react-i18next";
import useSWR from "swr";
import { SettingsPage, SettingsSection } from "@/components/settings-layout";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ErrorNotice } from "@/components/ui/page-layout";
import { Skeleton } from "@/components/ui/skeleton";
import {
	connectionKey,
	getConnection,
	saveConnection,
	testConnection,
	type OpenVikingConnection,
} from "./api";
import { knowledgeErrorMessage } from "./errors";

export function OpenVikingSettings() {
	const { t } = useTranslation();
	const connection = useSWR(connectionKey, getConnection, { shouldRetryOnError: false });
	return (
		<SettingsPage title={t("openviking.settings")}>
			<SettingsSection title={t("openviking.connection")}>
				{connection.error && (
					<ErrorNotice
						action={
							<Button variant="outline" onClick={() => void connection.mutate()}>
								{t("workspaces.retry")}
							</Button>
						}
					>
						{knowledgeErrorMessage(connection.error, t("openviking.error"))}
					</ErrorNotice>
				)}
				{connection.isLoading && <Skeleton className="h-32 w-full max-w-lg" />}
				{connection.data !== undefined && (
					<ConnectionForm
						saved={connection.data}
						onSaved={(value) => void connection.mutate(value, { revalidate: false })}
					/>
				)}
			</SettingsSection>
		</SettingsPage>
	);
}

function ConnectionForm({
	saved,
	onSaved,
}: {
	saved: OpenVikingConnection | null;
	onSaved(value: OpenVikingConnection): void;
}) {
	const { t } = useTranslation();
	const [draftURL, setDraftURL] = useState<string>();
	const [apiKey, setAPIKey] = useState("");
	const [pending, setPending] = useState<"test" | "save" | null>(null);
	const [feedback, setFeedback] = useState<{
		action: "test" | "save";
		connection: OpenVikingConnection;
	} | null>(null);
	const [error, setError] = useState("");
	const url = draftURL ?? saved?.url ?? "";

	function resetFeedback() {
		setError("");
		setFeedback(null);
	}

	async function submit(action: "test" | "save") {
		if (!url.trim() || pending) return;
		setPending(action);
		resetFeedback();
		const input = { url, apiKey: apiKey.trim() ? apiKey : undefined };
		try {
			const result = await (action === "test" ? testConnection(input) : saveConnection(input));
			if (action === "save") {
				onSaved(result);
				setDraftURL(undefined);
				setAPIKey("");
			}
			setFeedback({ action, connection: result });
		} catch (err) {
			setError(knowledgeErrorMessage(err, t("openviking.error")));
		} finally {
			setPending(null);
		}
	}

	return (
		<form
			className="grid max-w-lg gap-4"
			onSubmit={(event) => {
				event.preventDefault();
				void submit("save");
			}}
		>
			<div className="space-y-2">
				<Label htmlFor="openviking-url">{t("openviking.url")}</Label>
				<Input
					id="openviking-url"
					type="url"
					autoComplete="off"
					spellCheck={false}
					required
					placeholder="http://127.0.0.1:1933"
					value={url}
					disabled={pending !== null}
					onChange={(event) => {
						setDraftURL(event.target.value);
						resetFeedback();
					}}
				/>
			</div>
			<div className="space-y-2">
				<Label htmlFor="openviking-api-key">{t("openviking.apiKey")}</Label>
				<Input
					id="openviking-api-key"
					type="password"
					autoComplete="new-password"
					spellCheck={false}
					value={apiKey}
					disabled={pending !== null}
					placeholder={
						saved?.hasApiKey && saved.url === url.trim() ? t("openviking.keySaved") : undefined
					}
					onChange={(event) => {
						setAPIKey(event.target.value);
						resetFeedback();
					}}
				/>
				<p className="text-xs text-muted-foreground">{t("openviking.keyHint")}</p>
			</div>
			<div className="flex flex-wrap gap-2">
				<Button type="submit" disabled={!url.trim() || pending !== null}>
					{t(pending === "save" ? "openviking.saving" : "openviking.saveConnection")}
				</Button>
				<Button
					type="button"
					variant="outline"
					disabled={!url.trim() || pending !== null}
					onClick={() => void submit("test")}
				>
					{t(pending === "test" ? "openviking.testingConnection" : "openviking.testConnection")}
				</Button>
			</div>
			{error && <ErrorNotice>{error}</ErrorNotice>}
			{feedback && (
				<div role="status" className="space-y-1 text-sm">
					<p>
						{t(feedback.action === "test" ? "openviking.testPassed" : "openviking.connectionSaved")}
					</p>
					{feedback.connection.account && feedback.connection.user && (
						<p className="text-muted-foreground">
							{t("openviking.connectionIdentity", {
								account: feedback.connection.account,
								user: feedback.connection.user,
							})}
						</p>
					)}
				</div>
			)}
		</form>
	);
}
