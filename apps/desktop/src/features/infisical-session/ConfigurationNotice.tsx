import { LockKeyhole } from "lucide-react";
import { useTranslation } from "react-i18next";
import type { InfisicalConfiguration } from "@/api/session";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";

export function ConfigurationNotice({ configuration }: { configuration: InfisicalConfiguration }) {
	const { t } = useTranslation();
	return (
		<div className="flex flex-col gap-4">
			<div className="flex flex-wrap gap-2">
				<Badge variant="outline">{t(`server.modes.${configuration.mode}`)}</Badge>
				<Badge variant="secondary">{t(`server.configSources.${configuration.source}`)}</Badge>
			</div>
			{configuration.managed ? (
				<Alert role="status">
					<LockKeyhole aria-hidden="true" />
					<AlertTitle>{t("server.deploymentManaged")}</AlertTitle>
					<AlertDescription>
						<p>{t("server.deploymentManagedHint")}</p>
						<dl className="grid w-full gap-4 pt-3 ud-sm:grid-cols-2">
							<div className="flex min-w-0 flex-col gap-1">
								<dt>{t("session.site")}</dt>
								<dd className="break-all">{configuration.siteUrl}</dd>
							</div>
							<div className="flex min-w-0 flex-col gap-1">
								<dt>{t("server.clientID")}</dt>
								<dd className="break-all">{configuration.clientId}</dd>
							</div>
							{configuration.organizationId ? (
								<div className="flex min-w-0 flex-col gap-1 ud-sm:col-span-2">
									<dt>{t("session.organization")}</dt>
									<dd className="break-all">{configuration.organizationId}</dd>
								</div>
							) : null}
						</dl>
					</AlertDescription>
				</Alert>
			) : null}
		</div>
	);
}
