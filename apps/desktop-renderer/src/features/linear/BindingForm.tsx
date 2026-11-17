import { useState } from "react";
import { RefreshCw } from "lucide-react";
import { useTranslation } from "react-i18next";
import useSWR from "swr";
import { Button } from "@/components/ui/button";
import { ErrorNotice } from "@/components/ui/page-layout";
import { EnvironmentLink } from "@/features/environment-context/EnvironmentLink";
import {
	connectionKey,
	errorText,
	fetchJSON,
	projectKey,
	saveBinding,
	type ConnectionState,
	type Page,
	type Project,
	type Scope,
} from "./api";
import { Choice } from "./Choice";

export function BindingForm({ scope, onSaved }: { scope: Scope; onSaved(): void }) {
	const { t } = useTranslation();
	const connections = useSWR<ConnectionState>(connectionKey, fetchJSON);
	const [selectedConnection, setConnection] = useState("");
	const connection = selectedConnection || connections.data?.connections[0]?.id || "";
	return (
		<div className="space-y-4 rounded-lg border border-border bg-card p-5">
			<h2 className="font-medium">{t("linear.bind")}</h2>
			{connections.error && <ErrorNotice>{errorText(connections.error)}</ErrorNotice>}
			{!connections.data && !connections.error && <p role="status">{t("linear.loading")}</p>}
			{connections.data && !connection && (
				<>
					<p className="text-sm text-muted-foreground">{t("linear.connectFirst")}</p>
					<Button asChild>
						<EnvironmentLink to="/settings/linear">{t("linear.settings")}</EnvironmentLink>
					</Button>
				</>
			)}
			{connection && (
				<>
					<Choice
						label={t("linear.account")}
						value={connection}
						onChange={setConnection}
						options={connections.data!.connections.map((c) => ({
							id: c.id,
							name: `${c.organizationName} · ${c.name}`,
						}))}
					/>
					<ProjectPicker key={connection} connection={connection} scope={scope} onSaved={onSaved} />
				</>
			)}
		</div>
	);
}

function ProjectPicker({
	connection,
	scope,
	onSaved,
}: {
	connection: string;
	scope: Scope;
	onSaved(): void;
}) {
	const { t } = useTranslation();
	const [after, setAfter] = useState("");
	const [projectId, setProject] = useState("");
	const [team, setTeam] = useState("");
	const [busy, setBusy] = useState(false);
	const [refreshing, setRefreshing] = useState(false);
	const [error, setError] = useState("");
	const projects = useSWR<Page<Project>>(
		`/issue-tracking/projects?${new URLSearchParams({ connection, after })}`,
		fetchJSON,
	);
	const project = useSWR<Project>(projectId ? projectKey(connection, projectId) : null, fetchJSON);
	const selectedTeam =
		project.data?.teams.find((item) => item.id === team)?.id || project.data?.teams[0]?.id || "";
	async function refresh() {
		setRefreshing(true);
		setError("");
		try {
			await Promise.all([projects.mutate(), project.mutate()]);
		} catch (error) {
			setError(errorText(error));
		} finally {
			setRefreshing(false);
		}
	}
	return (
		<form
			className="space-y-4"
			onSubmit={(e) => {
				e.preventDefault();
				if (!project.data) return;
				setBusy(true);
				setError("");
				void saveBinding(scope, connection, project.data, selectedTeam)
					.then(onSaved)
					.catch((e) => setError(errorText(e)))
					.finally(() => setBusy(false));
			}}
		>
			<Choice
				label={t("linear.project")}
				value={projectId}
				onChange={(id) => {
					setProject(id);
					setTeam("");
				}}
				options={projects.data?.nodes || []}
				placeholder={t("linear.chooseProject")}
				disabled={busy}
			/>
			<div className="flex flex-wrap gap-2">
				<Button
					type="button"
					variant="outline"
					disabled={busy || refreshing}
					onClick={() => void refresh()}
				>
					<RefreshCw />
					{t("linear.refresh")}
				</Button>
				{after && (
					<Button
						type="button"
						variant="ghost"
						onClick={() => {
							setAfter("");
							setProject("");
						}}
					>
						{t("linear.firstPage")}
					</Button>
				)}
				{projects.data?.pageInfo.hasNextPage && (
					<Button
						type="button"
						variant="ghost"
						onClick={() => {
							setAfter(projects.data!.pageInfo.endCursor);
							setProject("");
						}}
					>
						{t("linear.nextProjects")}
					</Button>
				)}
			</div>
			{project.data && (
				<Choice
					label={t("linear.defaultTeam")}
					value={selectedTeam}
					onChange={setTeam}
					options={project.data.teams}
					disabled={busy}
				/>
			)}
			<p className="text-xs text-muted-foreground">{t("linear.projectTeamsHint")}</p>
			{(error || projects.error || project.error) && (
				<ErrorNotice>{error || errorText(projects.error || project.error)}</ErrorNotice>
			)}
			<Button type="submit" disabled={busy || refreshing || !selectedTeam}>
				{busy ? t("git.working") : t("linear.bind")}
			</Button>
		</form>
	);
}
