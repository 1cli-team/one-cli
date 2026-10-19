import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router-dom";
import { FolderOpen } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { invoke } from "@tauri-apps/api/core";
import { chooseWorkspace, native, useDesktop } from "./runtime";
import { useSessionSWRConfig } from "./cache";
export function OpenWorkspace() {
	const { t } = useTranslation();
	const [busy, setBusy] = useState(false);
	const navigate = useNavigate();
	const { mutate } = useSessionSWRConfig();
	async function open() {
		setBusy(true);
		try {
			const path = await chooseWorkspace();
			if (!path) return;
			const entry = await invoke<{ entryId: string }>("local_operation", {
				operation: "observe",
				payload: {
					path,
					serverURL: useDesktop.getState().serverURL,
					local: useDesktop.getState().localConnection,
				},
			});
			await mutate("/workspaces");
			navigate(`/workspace/${encodeURIComponent(entry.entryId)}`);
		} catch (e) {
			toast.error((e as Error).message || String(e));
		} finally {
			setBusy(false);
		}
	}
	if (!native) return null;
	return (
		<Button disabled={busy} onClick={() => void open()}>
			<FolderOpen />
			{t("desktop.openWorkspace")}
		</Button>
	);
}
