import { useId, useState } from "react";
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
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { ErrorNotice } from "@/components/ui/page-layout";
import { EnvironmentLink } from "@/features/environment-context/EnvironmentLink";
import { Choice } from "./Choice";
import {
	createAndBindProject,
	errorText,
	fetchJSON,
	teamsKey,
	type Binding,
	type Connection,
	type Named,
	type Scope,
} from "./api";

export function CreateLinearProjectDialog({
	scope,
	account,
	projectName,
	onClose,
	onSaved,
}: {
	scope: Scope;
	account: Connection;
	projectName: string;
	onClose(): void;
	onSaved(binding: Binding): void;
}) {
	const { t } = useTranslation();
	const nameId = useId();
	const [customName, setCustomName] = useState<string>();
	const [teamId, setTeamId] = useState("");
	const [busy, setBusy] = useState(false);
	const [error, setError] = useState<unknown>();
	const teams = useSWR<Named[]>(account.canCreateProjects ? teamsKey(account.id) : null, fetchJSON);
	const name = customName ?? projectName;
	const selected =
		teams.data?.find((team) => team.id === teamId) ||
		(teams.data?.length === 1 ? teams.data[0] : undefined);

	async function submit() {
		if (busy || !account.canCreateProjects || !selected || !name.trim()) return;
		setBusy(true);
		setError(undefined);
		try {
			onSaved(await createAndBindProject(scope, account.id, name.trim(), selected.id));
			onClose();
		} catch (error) {
			setError(error);
		} finally {
			setBusy(false);
		}
	}

	return (
		<Dialog
			open
			onOpenChange={(open) => {
				if (!open && !busy) onClose();
			}}
		>
			<DialogContent showCloseButton={!busy}>
				<DialogHeader>
					<DialogTitle>{t("linear.createProject")}</DialogTitle>
					<DialogDescription>
						{account.canCreateProjects
							? t("linear.createProjectDescription", { workspace: account.organizationName })
							: t("linear.createProjectAccessHint")}
					</DialogDescription>
				</DialogHeader>
				{account.canCreateProjects ? (
					<form
						className="flex flex-col gap-5"
						onSubmit={(event) => {
							event.preventDefault();
							void submit();
						}}
					>
						<FieldGroup className="gap-4">
							<Field data-disabled={busy}>
								<FieldLabel htmlFor={nameId}>{t("linear.projectName")}</FieldLabel>
								<Input
									id={nameId}
									value={name}
									onChange={(event) => setCustomName(event.target.value)}
									disabled={busy}
									required
								/>
							</Field>
							<Choice
								label={t("linear.team")}
								placeholder={t("linear.chooseTeam")}
								options={teams.data || []}
								value={selected?.id || ""}
								onChange={setTeamId}
								disabled={busy || !teams.data?.length}
							/>
						</FieldGroup>
						{!teams.data && !teams.error && <p role="status">{t("linear.loading")}</p>}
						{teams.data?.length === 0 && (
							<p role="status" className="text-sm text-muted-foreground">
								{t("linear.noTeams")}
							</p>
						)}
						{Boolean(error || teams.error) && (
							<ErrorNotice>{errorText(error || teams.error)}</ErrorNotice>
						)}
						<DialogFooter>
							<Button type="button" variant="outline" onClick={onClose} disabled={busy}>
								{t("git.cancel")}
							</Button>
							<Button type="submit" disabled={busy || !selected || !name.trim()}>
								{t(busy ? "git.working" : "linear.createAndBind")}
							</Button>
						</DialogFooter>
					</form>
				) : (
					<DialogFooter>
						<Button asChild>
							<EnvironmentLink to="/settings/linear">{t("linear.settings")}</EnvironmentLink>
						</Button>
					</DialogFooter>
				)}
			</DialogContent>
		</Dialog>
	);
}
