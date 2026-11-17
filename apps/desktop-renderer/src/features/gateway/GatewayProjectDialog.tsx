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
import { Skeleton } from "@/components/ui/skeleton";
import { Choice } from "@/features/linear/Choice";
import { errorText } from "@/features/linear/api";
import {
	gatewayRepositoriesKey,
	getProjectBindings,
	getProjectMapping,
	getProjectOrigins,
	getRepositories,
	mappingKey,
	projectBindingsKey,
	projectOriginsKey,
	repositoryLabel,
	repositoryRef,
	saveProjectMapping,
	type MappingInput,
	type ProjectBinding,
	type ProjectMapping,
	type ProjectOrigin,
	type RepositoryOption,
} from "./api";

function bindingID({ binding }: ProjectBinding) {
	return `${binding.scope.kind}:${binding.scope.id}`;
}

function projectRepositories(repositories: RepositoryOption[], origins: ProjectOrigin[]) {
	const seen = new Set<string>();
	return origins.flatMap((origin) => {
		const ref = repositoryRef(origin);
		if (!ref) return [{ origin, matches: [] as RepositoryOption[] }];
		const key = `${ref.host}/${ref.slug.toLowerCase()}`;
		if (seen.has(key)) return [];
		seen.add(key);
		const matches = repositories.filter((repo) => {
			const candidate = repositoryRef(repo);
			return (
				candidate?.host === ref.host && candidate.slug.toLowerCase() === ref.slug.toLowerCase()
			);
		});
		const samePath = matches.find((repo) => repo.path === origin.path);
		return [{ origin, matches: samePath ? [samePath] : matches }];
	});
}

export function GatewayProjectDialog({ onClose, onSaved }: { onClose(): void; onSaved(): void }) {
	const { t } = useTranslation();
	const bindings = useSWR(projectBindingsKey, getProjectBindings);
	const repositories = useSWR(gatewayRepositoriesKey, getRepositories, { dedupingInterval: 0 });
	const [projectId, setProjectId] = useState("");
	const [saving, setSaving] = useState(false);
	const binding = bindings.data?.find((item) => bindingID(item) === projectId);
	const mapping = useSWR(binding ? mappingKey(binding.binding.scope) : null, getProjectMapping);
	const origins = useSWR(
		binding ? projectOriginsKey(binding.binding.scope) : null,
		getProjectOrigins,
	);
	const error = bindings.error || repositories.error || mapping.error || origins.error;
	const isLoading =
		bindings.isLoading || repositories.isLoading || mapping.isLoading || origins.isLoading;
	const workspaces =
		repositories.data && origins.data ? projectRepositories(repositories.data, origins.data) : [];
	const projectOptions =
		bindings.data?.map((item) => ({
			id: bindingID(item),
			name:
				item.name === item.binding.project.name
					? item.name
					: `${item.name} · ${item.binding.project.name}`,
		})) ?? [];

	async function save(input: MappingInput) {
		if (!binding || saving) return;
		setSaving(true);
		try {
			const updated = await saveProjectMapping(binding.binding.scope, input);
			void mapping.mutate(updated, { revalidate: false });
			onSaved();
			onClose();
		} finally {
			setSaving(false);
		}
	}

	return (
		<Dialog open onOpenChange={(open) => !open && !saving && onClose()}>
			<DialogContent className="ud-sm:max-w-[44rem]" showCloseButton={!saving}>
				<DialogHeader>
					<DialogTitle>{t("gateway.configureProject")}</DialogTitle>
					<DialogDescription>{t("gateway.configureDescription")}</DialogDescription>
				</DialogHeader>
				{error && <ErrorNotice>{errorText(error)}</ErrorNotice>}
				{bindings.data && projectOptions.length === 0 && (
					<p className="text-sm text-muted-foreground">{t("gateway.noProjectBindings")}</p>
				)}
				{projectOptions.length > 0 && (
					<Choice
						label={t("gateway.linearProject")}
						placeholder={t("gateway.chooseProject")}
						value={projectId}
						options={projectOptions}
						onChange={setProjectId}
						disabled={saving}
					/>
				)}
				{binding && !workspaces.length && !isLoading && !error && (
					<p className="text-sm text-muted-foreground">{t("gateway.originMissing")}</p>
				)}
				{isLoading && <Skeleton className="h-12 w-full" aria-busy="true" />}
				{binding && mapping.data !== undefined && workspaces.length > 0 && !error ? (
					<MappingForm
						key={projectId}
						workspaces={workspaces}
						mapping={mapping.data}
						saving={saving}
						onSave={save}
						onClose={onClose}
					/>
				) : (
					<DialogFooter>
						<Button type="button" variant="outline" onClick={onClose} disabled={saving}>
							{t("form.close")}
						</Button>
					</DialogFooter>
				)}
			</DialogContent>
		</Dialog>
	);
}

function MappingForm({
	workspaces,
	mapping,
	saving,
	onSave,
	onClose,
}: {
	workspaces: ReturnType<typeof projectRepositories>;
	mapping: ProjectMapping | null;
	saving: boolean;
	onSave(input: MappingInput): Promise<void>;
	onClose(): void;
}) {
	const { t } = useTranslation();
	const [selected, setSelected] = useState<Record<string, string>>({});
	const [saveError, setSaveError] = useState("");
	const savedIDs = mapping?.repositoryIds ?? [mapping?.repositoryId];
	const rows = workspaces.map(({ origin, matches }) => {
		const explicit = matches.find((repo) => repo.id === selected[origin.path]);
		const saved = matches.find((repo) => savedIDs.includes(repo.id));
		const repository = explicit ?? saved ?? (matches.length === 1 ? matches[0] : undefined);
		return { origin, matches, repository };
	});
	const ready = rows.every(({ repository }) => !!repository);
	async function save() {
		if (!ready || saving) return;
		setSaveError("");
		try {
			const repositoryIds = rows.flatMap(({ repository }) => (repository ? [repository.id] : []));
			await onSave(
				repositoryIds.length === 1 ? { repositoryId: repositoryIds[0] } : { repositoryIds },
			);
		} catch (err) {
			setSaveError(errorText(err));
		}
	}

	return (
		<form
			className="grid gap-4"
			onSubmit={(event) => {
				event.preventDefault();
				void save();
			}}
		>
			<ul
				className="grid max-h-[45vh] gap-3 overflow-y-auto"
				aria-label={t("gateway.projectRepositories")}
			>
				{rows.map(({ origin, matches, repository }) => (
					<li key={origin.path} className="grid gap-2 rounded-md border border-border p-3">
						{matches.length > 1 ? (
							<Choice
								label={origin.name || t("gateway.repository")}
								placeholder={t("gateway.chooseRepository")}
								value={repository?.id ?? ""}
								options={matches.map((repo) => ({ id: repo.id, name: repo.path }))}
								onChange={(id) => {
									setSelected((current) => ({ ...current, [origin.path]: id }));
									setSaveError("");
								}}
								disabled={saving}
							/>
						) : (
							<p className="text-xs text-muted-foreground">
								{origin.name || t("gateway.repository")}
							</p>
						)}
						{repository ? (
							<>
								<p className="break-all text-sm font-medium">{repositoryLabel(repository)}</p>
								{matches.length <= 1 && (
									<p className="break-all font-mono text-xs text-muted-foreground">
										{repository.path}
									</p>
								)}
							</>
						) : matches.length === 0 ? (
							<p className="text-sm text-muted-foreground">
								{t(
									origin.repository
										? "gateway.originExecutionHost"
										: origin.github
											? "gateway.originCheckoutMissing"
											: origin.hasOrigin
												? "gateway.originProviderUnsupported"
												: "gateway.originMissing",
								)}
							</p>
						) : null}
					</li>
				))}
			</ul>
			{saveError && <ErrorNotice>{saveError}</ErrorNotice>}
			<DialogFooter>
				<Button type="button" variant="outline" onClick={onClose} disabled={saving}>
					{t("git.cancel")}
				</Button>
				<Button type="submit" disabled={!ready || saving}>
					{t(saving ? "gateway.saving" : "gateway.save")}
				</Button>
			</DialogFooter>
		</form>
	);
}
