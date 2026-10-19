import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { FieldGroup, Field, FieldLabel, FieldDescription } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Spinner } from "@/components/ui/spinner";
import { http } from "@/lib/http";

export function MachineIdentityForm({ onConnected }: { onConnected: () => Promise<unknown> }) {
	const { t } = useTranslation();
	const [site, setSite] = useState("https://app.infisical.com");
	const [clientID, setClientID] = useState("");
	const [secret, setSecret] = useState("");
	const [organization, setOrganization] = useState("");
	const [busy, setBusy] = useState(false);
	const [error, setError] = useState("");
	async function submit(event: React.FormEvent) {
		event.preventDefault();
		setBusy(true);
		setError("");
		try {
			await http.post("/session/machine", {
				siteUrl: site,
				clientId: clientID,
				clientSecret: secret,
				organizationId: organization,
			});
			setSecret("");
			await onConnected();
		} catch (cause) {
			setError((cause as Error).message);
		} finally {
			setBusy(false);
		}
	}
	return (
		<form className="flex max-w-xl flex-col gap-6" onSubmit={(event) => void submit(event)}>
			<FieldGroup>
				<Field>
					<FieldLabel htmlFor="machine-site">{t("session.site")}</FieldLabel>
					<Input
						id="machine-site"
						type="url"
						required
						value={site}
						onChange={(event) => setSite(event.target.value)}
						disabled={busy}
					/>
					<FieldDescription>{t("server.machineHint")}</FieldDescription>
				</Field>
				<Field>
					<FieldLabel htmlFor="machine-client">{t("server.clientID")}</FieldLabel>
					<Input
						id="machine-client"
						required
						value={clientID}
						onChange={(event) => setClientID(event.target.value)}
						disabled={busy}
					/>
				</Field>
				<Field>
					<FieldLabel htmlFor="machine-secret">{t("server.clientSecret")}</FieldLabel>
					<Input
						id="machine-secret"
						type="password"
						autoComplete="new-password"
						required
						value={secret}
						onChange={(event) => setSecret(event.target.value)}
						disabled={busy}
					/>
				</Field>
				<Field>
					<FieldLabel htmlFor="machine-organization">{t("server.organizationID")}</FieldLabel>
					<Input
						id="machine-organization"
						value={organization}
						onChange={(event) => setOrganization(event.target.value)}
						disabled={busy}
					/>
				</Field>
			</FieldGroup>
			<Button type="submit" disabled={busy}>
				{busy ? <Spinner /> : null}
				{t("server.connectMachine")}
			</Button>
			{error ? (
				<p role="alert" className="break-words text-sm text-error-foreground">
					{error}
				</p>
			) : null}
		</form>
	);
}
