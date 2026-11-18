import { useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { SettingsPage, SettingsSection } from "@/components/settings-layout";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ErrorNotice } from "@/components/ui/page-layout";
import { Skeleton } from "@/components/ui/skeleton";
import { EnvironmentLink } from "@/features/environment-context/EnvironmentLink";
import type { HttpError } from "@/types/api";
import { ConnectionBadge } from "./ConnectionBadge";
import { saveConnection, testConnection, useGatewayStatus, type GatewayStatus } from "./api";

export function GatewaySettings() {
	const { t } = useTranslation();
	const status = useGatewayStatus();

	return (
		<SettingsPage
			title={t("gateway.settings")}
			status={status.data && <ConnectionBadge state={status.data.state} />}
		>
			<SettingsSection>
				{status.error && <ErrorNotice>{t("gateway.statusError")}</ErrorNotice>}
				{status.data ? (
					<ConnectionForm
						saved={status.data}
						onSaved={(result) => void status.mutate(result, { revalidate: false })}
					/>
				) : status.isLoading ? (
					<div className="space-y-3" aria-busy="true">
						<Skeleton className="h-9 w-full max-w-lg" />
						<Skeleton className="h-9 w-full max-w-lg" />
					</div>
				) : (
					<Button variant="outline" className="self-start" onClick={() => void status.mutate()}>
						{t("gateway.refresh")}
					</Button>
				)}
			</SettingsSection>
		</SettingsPage>
	);
}

function ConnectionForm({
	saved,
	onSaved,
}: {
	saved: GatewayStatus;
	onSaved(result: GatewayStatus): void;
}) {
	const { t } = useTranslation();
	const [draft, setDraft] = useState<{ url: string; token: string } | null>(null);
	const { url, token } = draft ?? { url: saved.url, token: "" };
	const [pending, setPending] = useState<"save" | "test" | null>(null);
	const [error, setError] = useState("");
	const [testResult, setTestResult] = useState<GatewayStatus | null>(null);
	const [didSave, setDidSave] = useState(false);
	const canReuseToken = saved.hasToken && url.trim() === saved.url;
	const canCheck = !!url.trim() && (!!token.trim() || canReuseToken);
	const canSave = canCheck && (!canReuseToken || !!token.trim());

	function resetFeedback() {
		setError("");
		setTestResult(null);
		setDidSave(false);
	}

	async function submit(action: "save" | "test", event?: FormEvent<HTMLFormElement>) {
		event?.preventDefault();
		if (pending || (action === "test" ? !canCheck : !canSave)) return;
		resetFeedback();
		setPending(action);
		try {
			if (action === "test") {
				setTestResult(await testConnection({ url, token }));
			} else {
				const result = await saveConnection({ url, token });
				setDraft(null);
				onSaved(result);
				setDidSave(true);
			}
		} catch (e) {
			setError((e as HttpError).message || t("gateway.statusError"));
		} finally {
			setPending(null);
		}
	}

	return (
		<form className="grid max-w-lg gap-4" onSubmit={(event) => void submit("save", event)}>
			<div className="space-y-2">
				<Label htmlFor="gateway-url">{t("gateway.address")}</Label>
				<Input
					id="gateway-url"
					type="url"
					autoComplete="off"
					spellCheck={false}
					placeholder="http://127.0.0.1:39128"
					aria-describedby="gateway-address-hint"
					value={url}
					required
					disabled={!!pending}
					onChange={(event) => {
						setDraft({ url: event.target.value, token });
						resetFeedback();
					}}
				/>
				<p id="gateway-address-hint" className="text-xs text-muted-foreground">
					{t("gateway.addressHint")}
				</p>
			</div>
			<div className="space-y-2">
				<Label htmlFor="gateway-token">{t("gateway.token")}</Label>
				<Input
					id="gateway-token"
					type="password"
					autoComplete="new-password"
					spellCheck={false}
					value={token}
					required={!canReuseToken}
					disabled={!!pending}
					placeholder={canReuseToken ? t("gateway.tokenSaved") : undefined}
					onChange={(event) => {
						setDraft({ url, token: event.target.value });
						resetFeedback();
					}}
				/>
			</div>
			<div className="flex flex-wrap items-center gap-2">
				<Button type="submit" disabled={!!pending || !canSave}>
					{t(pending === "save" ? "gateway.saving" : "gateway.save")}
				</Button>
				<Button
					type="button"
					variant="outline"
					disabled={!!pending || !canCheck}
					onClick={() => void submit("test")}
				>
					{t(pending === "test" ? "gateway.testing" : "gateway.test")}
				</Button>
				<Button type="button" variant="ghost" asChild>
					<EnvironmentLink to="/gateway">{t("gateway.viewMonitor")}</EnvironmentLink>
				</Button>
			</div>
			{saved.state === "invalid" && <ErrorNotice>{t("gateway.invalidHint")}</ErrorNotice>}
			{error && <ErrorNotice>{error}</ErrorNotice>}
			{testResult && (
				<div className="flex flex-wrap items-center gap-2 text-sm">
					<span>{t("gateway.testResult")}</span>
					<ConnectionBadge state={testResult.state} />
				</div>
			)}
			{didSave && (
				<p role="status" className="text-sm text-muted-foreground">
					{t("gateway.saved")}
				</p>
			)}
		</form>
	);
}
