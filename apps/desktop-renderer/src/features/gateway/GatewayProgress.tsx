import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { errorText } from "@/features/linear/api";
import { continueExecution, type ExecutionProgress, type ExecutionState } from "./executions";
import { GatewayMarkdown } from "./GatewayMarkdown";

export function GatewayProgress({
	jobId,
	progress,
	state,
	onContinued,
}: {
	jobId: string;
	progress: ExecutionProgress;
	state: ExecutionState;
	onContinued(progress: ExecutionProgress): void;
}) {
	const { t } = useTranslation();
	const [busy, setBusy] = useState(false);
	const [error, setError] = useState("");
	const waiting = state === "waiting_to_continue";

	async function resume() {
		if (busy || !waiting || progress.continueRequested) return;
		setBusy(true);
		setError("");
		try {
			onContinued(await continueExecution(jobId, progress.id));
		} catch (failure) {
			setError(errorText(failure));
		} finally {
			setBusy(false);
		}
	}

	return (
		<section className="grid gap-2 rounded-md border border-border p-3 text-sm">
			<h3 className="font-medium">{t("gateway.execution.progress.title")}</h3>
			<GatewayMarkdown value={progress.summary} />
			{(waiting || state === "queued") && progress.continueRequested && (
				<p role="status" className="text-muted-foreground">
					{t("gateway.execution.progress.requested")}
				</p>
			)}
			{waiting && !progress.continueRequested && (
				<Button
					size="sm"
					className="justify-self-end"
					disabled={busy}
					onClick={() => void resume()}
				>
					{t(busy ? "gateway.execution.progress.sending" : "gateway.execution.progress.continue")}
				</Button>
			)}
			{error && (
				<p role="alert" className="wrap-anywhere text-destructive">
					{error}
				</p>
			)}
		</section>
	);
}
