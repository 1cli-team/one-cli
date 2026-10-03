import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { RefreshCw } from "lucide-react";
import { useSWRConfig } from "swr";
import {
	initializeGlobalLocation,
	message,
	sessionKey,
	type SessionState,
	type SessionInfo,
	type SharedCredentialsState,
} from "@/api/session";
import { Button } from "@/components/ui/button";
import { ErrorNotice } from "@/components/ui/page-layout";
import { Spinner } from "@/components/ui/spinner";

export function SharedCredentialsNotice({
	state,
	account,
}: {
	state?: SharedCredentialsState | null;
	account: SessionInfo;
}) {
	const { t } = useTranslation();
	const { mutate } = useSWRConfig();
	const [busy, setBusy] = useState(false);
	const [error, setError] = useState("");
	const active = useRef(true);
	useEffect(() => {
		active.current = true;
		return () => {
			active.current = false;
		};
	}, []);
	async function retry() {
		if (busy) return;
		setBusy(true);
		setError("");
		try {
			const result = await initializeGlobalLocation();
			if (!active.current) return;
			await mutate<SessionState>(
				sessionKey,
				(current) =>
					current?.session.loggedIn &&
					current.session.siteUrl === account.siteUrl &&
					current.session.userId === account.userId &&
					current.session.organizationId === account.organizationId &&
					current.session.expiresAt === account.expiresAt
						? { ...current, sharedCredentials: result.sharedCredentials }
						: current,
				{ revalidate: true },
			);
		} catch (cause) {
			if (active.current) setError(message(cause));
		} finally {
			if (active.current) setBusy(false);
		}
	}
	if (state?.status === "ready") return null;
	if (!state || state.status === "preparing") {
		return (
			<p role="status" className="flex items-center gap-2 text-sm text-muted-foreground">
				<Spinner />
				{t("global.preparing")}
			</p>
		);
	}
	return (
		<ErrorNotice
			action={
				<Button variant="outline" size="sm" disabled={busy} onClick={() => void retry()}>
					{busy ? <Spinner /> : <RefreshCw />}
					{t("secrets.retry")}
				</Button>
			}
		>
			<p>{t("global.preparationFailed")}</p>
			<p className="mt-1 break-words">{error || state.error}</p>
		</ErrorNotice>
	);
}
