import { useState } from "react";
import { useTranslation } from "react-i18next";
import useSWR from "swr";
import { Button } from "@/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import { ErrorNotice } from "@/components/ui/page-layout";
import { connectionKey, getConnection } from "@/features/openviking/api";
import { knowledgeErrorMessage } from "@/features/openviking/errors";
import { syncWorkerOpenViking, type ExecutionWorker } from "./executions";

export function GatewayOpenVikingSyncDialog({
	connection,
	worker,
	onClose,
}: {
	connection: string;
	worker: ExecutionWorker;
	onClose(): void;
}) {
	const { t } = useTranslation();
	const saved = useSWR(connectionKey, getConnection, { shouldRetryOnError: false });
	const [pending, setPending] = useState(false);
	const [synced, setSynced] = useState(false);
	const [error, setError] = useState("");

	async function sync() {
		if (!saved.data || pending || synced) return;
		setPending(true);
		setError("");
		try {
			await syncWorkerOpenViking(connection, worker.id);
			setSynced(true);
		} catch (err) {
			setError(knowledgeErrorMessage(err, t("gateway.execution.openViking.failed")));
		} finally {
			setPending(false);
		}
	}

	return (
		<Dialog open onOpenChange={(open) => !open && !pending && onClose()}>
			<DialogContent showCloseButton={!pending}>
				<DialogHeader>
					<DialogTitle>{t("gateway.execution.openViking.title")}</DialogTitle>
					<DialogDescription>
						{t("gateway.execution.openViking.description", { worker: worker.name })}
					</DialogDescription>
				</DialogHeader>
				{saved.data && <p className="break-all text-sm text-muted-foreground">{saved.data.url}</p>}
				{saved.error && <ErrorNotice>{t("gateway.execution.openViking.loadFailed")}</ErrorNotice>}
				{saved.data === null && (
					<p className="text-sm">{t("gateway.execution.openViking.localRequired")}</p>
				)}
				{error && <ErrorNotice>{error}</ErrorNotice>}
				{synced && (
					<p role="status" className="text-sm">
						{t("gateway.execution.openViking.success", { worker: worker.name })}
					</p>
				)}
				<DialogFooter>
					<Button variant="outline" disabled={pending} onClick={onClose}>
						{t(synced ? "form.close" : "form.cancel")}
					</Button>
					{!synced && (
						<Button disabled={!saved.data || pending} onClick={() => void sync()}>
							{t(
								pending
									? "gateway.execution.openViking.pending"
									: "gateway.execution.openViking.action",
							)}
						</Button>
					)}
				</DialogFooter>
			</DialogContent>
		</Dialog>
	);
}
