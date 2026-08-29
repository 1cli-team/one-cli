// SectionDetail renders one catalog-backed profile section. Backend identity,
// defaults and fields come from GET /api/catalog, so adding an adapter does not
// require another switch in the Dashboard.

import { Check, Eye, EyeOff, Loader2, Plus, Star, Trash2 } from "lucide-react";
import type React from "react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useParams } from "react-router-dom";
import useSWR from "swr";
import { humanizeBackendName, useBackendCatalog } from "@/api/catalog";
import { getSection, removeProfile, sectionKey, setDefault } from "@/api/configure";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "@/components/ui/table";
import {
	emptyProfile,
	ProfileEditorDialog,
	type ProfileEditorTarget,
	ProfileSummary,
} from "@/features/profile-editor/ProfileEditorDialog";
import { useToast } from "@/hooks/useToast";
import type { BackendSpec, SectionKey } from "@/types/api";

function backendTitle(t: ReturnType<typeof useTranslation>["t"], backend: BackendSpec): string {
	return t(`sections.${backend.domain}.${backend.name}.title`, {
		defaultValue: humanizeBackendName(backend.name),
	});
}

function backendDescription(
	t: ReturnType<typeof useTranslation>["t"],
	backend: BackendSpec,
): string {
	return t(`sections.${backend.domain}.${backend.name}.description`, {
		defaultValue: backend.id,
	});
}

export const SectionDetail: React.FC = () => {
	const params = useParams<{ domain: string; backend: string }>();
	const catalog = useBackendCatalog();
	const pair = `${params.domain ?? ""}/${params.backend ?? ""}` as SectionKey;
	const backend = catalog.byID.get(pair);
	const toast = useToast();
	const { t } = useTranslation();
	const [reveal, setReveal] = useState(false);
	const [editorTarget, setEditorTarget] = useState<ProfileEditorTarget | null>(null);

	const swrKey = backend ? sectionKey(backend.domain, backend.name, reveal) : null;
	const { data, error, isLoading, mutate } = useSWR(swrKey, () => {
		if (!backend) return Promise.reject(new Error("unknown section"));
		return getSection(backend.domain, backend.name, reveal);
	});

	if (catalog.error) {
		return (
			<Alert variant="destructive">
				<AlertTitle>{t("home.loadFailedTitle")}</AlertTitle>
				<AlertDescription>{catalog.error.message}</AlertDescription>
			</Alert>
		);
	}
	if (catalog.isLoading) {
		return (
			<div className="flex items-center gap-2 text-sm text-muted-foreground">
				<Loader2 className="h-4 w-4 animate-spin" /> {t("detail.loading")}
			</div>
		);
	}
	if (!backend || !backend.profile.configurable) {
		return (
			<Alert variant="destructive">
				<AlertTitle>{t("detail.unknownSectionTitle")}</AlertTitle>
				<AlertDescription>
					{t("detail.unknownSectionBody", {
						domain: params.domain,
						backend: params.backend,
					})}
				</AlertDescription>
			</Alert>
		);
	}
	const selectedBackend = backend;

	const refresh = () => mutate();

	async function onUse(name: string) {
		try {
			await setDefault(selectedBackend.domain, selectedBackend.name, name);
			toast.success(t("toast.setDefault", { name }));
			void refresh();
		} catch (err) {
			const e = err as { code?: string; message: string };
			toast.error(e.message, { description: e.code });
		}
	}

	async function onRemove(name: string) {
		if (!window.confirm(t("detail.confirmRemove", { name }))) return;
		try {
			await removeProfile(selectedBackend.domain, selectedBackend.name, name);
			toast.success(t("toast.removed", { name }));
			void refresh();
		} catch (err) {
			const e = err as { code?: string; message: string };
			toast.error(e.message, { description: e.code });
		}
	}

	const profiles = data?.section.profiles ?? {};
	const defaultName = data?.section.default ?? "";
	const profileNames = Object.keys(profiles).sort();
	const title = backendTitle(t, backend);
	const description = backendDescription(t, backend);

	return (
		<div className="space-y-5">
			<div className="flex flex-wrap items-center justify-between gap-3">
				<div className="min-w-0">
					<h1 className="text-xl font-semibold tracking-tight">{title}</h1>
					<p className="text-sm text-muted-foreground">{description}</p>
				</div>
				<div className="flex items-center gap-2">
					<Button
						variant="outline"
						size="sm"
						onClick={() => setReveal((value) => !value)}
						title={reveal ? t("detail.hideSecrets") : t("detail.showSecrets")}
					>
						{reveal ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
						{reveal ? t("detail.showingSecrets") : t("detail.showSecrets")}
					</Button>
					{editorTarget === null ? (
						<Button
							size="sm"
							onClick={() =>
								setEditorTarget({
									backend,
									name: "",
									profile: emptyProfile(backend),
									mode: "add",
									hasDefault: defaultName !== "",
								})
							}
						>
							<Plus className="h-4 w-4" /> {t("detail.addProfile")}
						</Button>
					) : null}
				</div>
			</div>

			{error ? (
				<Alert variant="destructive">
					<AlertTitle>{t("detail.loadFailedTitle")}</AlertTitle>
					<AlertDescription>{error.message}</AlertDescription>
				</Alert>
			) : null}

			<ProfileEditorDialog
				target={editorTarget}
				onOpenChange={(open) => {
					if (!open) setEditorTarget(null);
				}}
				onSaved={() => {
					void refresh();
				}}
			/>

			<div className="space-y-3">
				{isLoading ? (
					<div className="flex items-center gap-2 text-sm text-muted-foreground">
						<Loader2 className="h-4 w-4 animate-spin" /> {t("detail.loading")}
					</div>
				) : null}
				{!isLoading && profileNames.length === 0 ? (
					<Card>
						<CardContent className="py-8 text-center text-sm text-muted-foreground">
							{t("detail.empty")}
						</CardContent>
					</Card>
				) : null}
				{profileNames.length > 0 ? (
					<Card>
						<CardContent className="p-0">
							<div className="overflow-x-auto">
								<Table>
									<TableHeader>
										<TableRow>
											<TableHead className="min-w-48">{t("detail.tableProfile")}</TableHead>
											<TableHead className="min-w-72">{t("detail.tableSummary")}</TableHead>
											<TableHead className="min-w-72 text-right">
												{t("detail.tableActions")}
											</TableHead>
										</TableRow>
									</TableHeader>
									<TableBody>
										{profileNames.map((name) => {
											const profile = profiles[name];
											return (
												<TableRow key={name}>
													<TableCell>
														<div className="flex min-w-0 items-center gap-2">
															<span className="truncate font-medium">{name}</span>
															{name === defaultName ? (
																<Badge>
																	<Star className="h-3 w-3" /> {t("detail.default")}
																</Badge>
															) : null}
														</div>
													</TableCell>
													<TableCell className="text-muted-foreground">
														<ProfileSummary backend={backend} profile={profile} />
													</TableCell>
													<TableCell>
														<div className="flex items-center justify-end gap-1.5">
															{name !== defaultName ? (
																<Button size="sm" variant="outline" onClick={() => onUse(name)}>
																	<Check className="h-4 w-4" /> {t("detail.setDefault")}
																</Button>
															) : null}
															<Button
																size="sm"
																variant="outline"
																onClick={() =>
																	setEditorTarget({
																		backend,
																		name,
																		profile,
																		mode: "edit",
																		hasDefault: defaultName !== "",
																	})
																}
															>
																{t("detail.edit")}
															</Button>
															<Button
																size="sm"
																variant="destructive"
																onClick={() => onRemove(name)}
															>
																<Trash2 className="h-4 w-4" /> {t("detail.remove")}
															</Button>
														</div>
													</TableCell>
												</TableRow>
											);
										})}
									</TableBody>
								</Table>
							</div>
						</CardContent>
					</Card>
				) : null}
			</div>
		</div>
	);
};
