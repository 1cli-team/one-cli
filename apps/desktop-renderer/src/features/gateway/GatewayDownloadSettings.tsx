import { useId, useState } from "react";
import { useTranslation } from "react-i18next";
import { SettingsSection } from "@/components/settings-layout";
import { Checkbox } from "@/components/ui/checkbox";
import { ErrorNotice } from "@/components/ui/page-layout";
import { Spinner } from "@/components/ui/spinner";
import type { HttpError } from "@/types/api";
import { setGatewayDownloadSettings, useGatewayDownloadSettings, useGatewayStatus } from "./api";

export function GatewayDownloadSettings() {
	const { t } = useTranslation();
	const checkboxId = useId();
	const descriptionId = useId();
	const connection = useGatewayStatus();
	const settings = useGatewayDownloadSettings(connection.data);
	const [saving, setSaving] = useState(false);
	const [saveError, setSaveError] = useState("");
	const connected = connection.data?.state === "connected";

	async function change(chinaMirror: boolean) {
		if (!connected || !settings.data || settings.error || saving) return;
		setSaving(true);
		setSaveError("");
		try {
			await settings.mutate(setGatewayDownloadSettings(chinaMirror), { revalidate: false });
		} catch (err) {
			setSaveError((err as HttpError)?.message || t("gateway.downloads.saveError"));
			void settings.mutate();
		} finally {
			setSaving(false);
		}
	}

	return (
		<SettingsSection title={t("gateway.downloads.title")}>
			<div className="flex items-start justify-between gap-4">
				<div className="min-w-0 space-y-1">
					<label htmlFor={checkboxId} className="block text-sm font-medium">
						{t("gateway.downloads.useChinaMirror")}
					</label>
					<p id={descriptionId} className="text-xs text-muted-foreground">
						{t("gateway.downloads.hint")}
					</p>
				</div>
				<div className="flex items-center gap-2 pt-1">
					{saving && <Spinner aria-label={t("gateway.downloads.saving")} />}
					<Checkbox
						id={checkboxId}
						aria-describedby={descriptionId}
						checked={settings.data?.chinaMirror ?? false}
						disabled={!connected || !settings.data || !!settings.error || saving}
						onCheckedChange={(checked) => void change(checked === true)}
					/>
				</div>
			</div>
			{!connected && (connection.data || connection.error) && (
				<p className="text-xs text-muted-foreground">{t("gateway.downloads.connect")}</p>
			)}
			{settings.error?.status === 404 ? (
				<p className="text-xs text-muted-foreground">{t("gateway.downloads.updateRequired")}</p>
			) : (
				(saveError || settings.error) && (
					<ErrorNotice>{saveError || t("gateway.downloads.loadError")}</ErrorNotice>
				)
			)}
		</SettingsSection>
	);
}
