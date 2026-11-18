import { FileUp } from "lucide-react";
import { useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import useSWR from "swr";
import { Button } from "@/components/ui/button";
import { ErrorNotice } from "@/components/ui/page-layout";
import { knowledgeErrorMessage } from "./errors";
import {
	getImportStatus,
	importKnowledge,
	MAX_KNOWLEDGE_IMPORT_SIZE,
	type KnowledgeImport,
} from "./api";

const processing = (status: KnowledgeImport["status"]) =>
	status === "pending" || status === "running" || status === "cancelling";

export function ImportDocument({
	endpoint,
	onSearch,
	searchBusy,
}: {
	endpoint: string;
	onSearch(name: string): void;
	searchBusy: boolean;
}) {
	const { t } = useTranslation();
	const picker = useRef<HTMLInputElement>(null);
	const [submission, setSubmission] = useState<{ name: string; task: KnowledgeImport } | null>(
		null,
	);
	const [uploading, setUploading] = useState(false);
	const [uploadError, setUploadError] = useState("");
	const status = useSWR(
		submission ? ([endpoint, "openviking-import", submission.task.taskId] as const) : null,
		([key, , taskId]) => getImportStatus(key, taskId),
		{
			refreshInterval: (current) => (!current || processing(current.status) ? 2000 : 0),
			revalidateOnFocus: false,
			shouldRetryOnError: false,
		},
	);
	const task = status.data ?? submission?.task;

	async function upload(file: File) {
		setUploadError("");
		if (file.size === 0 || file.size > MAX_KNOWLEDGE_IMPORT_SIZE) {
			setUploadError(t("openviking.invalidImportFile"));
			return;
		}
		setUploading(true);
		try {
			setSubmission({ name: file.name, task: await importKnowledge(endpoint, file) });
		} catch (error) {
			setUploadError(knowledgeErrorMessage(error, t("openviking.error")));
		} finally {
			setUploading(false);
		}
	}

	return (
		<div className="space-y-2">
			<input
				ref={picker}
				type="file"
				className="hidden"
				onChange={(event) => {
					const file = event.currentTarget.files?.[0];
					event.currentTarget.value = "";
					if (file) void upload(file);
				}}
			/>
			<Button
				type="button"
				variant="outline"
				onClick={() => picker.current?.click()}
				disabled={uploading || Boolean(task && processing(task.status) && !status.error)}
			>
				<FileUp />
				{t(uploading ? "openviking.uploading" : "openviking.importDocument")}
			</Button>
			{uploadError && <ErrorNotice>{uploadError}</ErrorNotice>}
			{submission && task && !uploading && (
				<div className="flex flex-wrap items-center justify-between gap-3 rounded-lg border bg-card p-3 text-sm">
					<div className="min-w-0 space-y-1">
						<p className="font-medium break-words">{submission.name}</p>
						<p className="text-muted-foreground" role="status">
							{t(
								status.error
									? "openviking.importStatusUnavailable"
									: processing(task.status)
										? "openviking.indexing"
										: "openviking.importStatus." + task.status,
							)}
						</p>
						{task.error && <p className="break-words text-muted-foreground">{task.error}</p>}
					</div>
					{status.error ? (
						<Button type="button" variant="outline" size="sm" onClick={() => void status.mutate()}>
							{t("workspaces.retry")}
						</Button>
					) : (
						task.status === "completed" && (
							<Button
								type="button"
								variant="outline"
								size="sm"
								disabled={searchBusy}
								onClick={() => onSearch(submission.name)}
							>
								{t("openviking.searchImported")}
							</Button>
						)
					)}
				</div>
			)}
		</div>
	);
}
