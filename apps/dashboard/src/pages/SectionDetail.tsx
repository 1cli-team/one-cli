// SectionDetail renders one catalog-backed Settings section. Backend identity,
// defaults and fields come from GET /api/catalog, so adding an adapter does not
// require another switch in the Dashboard.

import { Check, Eye, EyeOff, Plus, Star, Trash2 } from "lucide-react";
import type React from "react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useParams } from "react-router-dom";
import useSWR from "swr";
import { humanizeBackendName, useBackendCatalog } from "@/api/catalog";
import { getSection, removeProfile, sectionKey, setDefault } from "@/api/configure";
import {
	AlertDialog,
	AlertDialogAction,
	AlertDialogCancel,
	AlertDialogContent,
	AlertDialogDescription,
	AlertDialogFooter,
	AlertDialogHeader,
	AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Empty, EmptyDescription, EmptyHeader } from "@/components/ui/empty";
import { Spinner } from "@/components/ui/spinner";
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
	return <SectionDetailContent domain={params.domain ?? ""} backendName={params.backend ?? ""} />;
};

export const SectionDetailContent: React.FC<{
	domain: string;
	backendName: string;
	embedded?: boolean;
}> = ({ domain, backendName, embedded = false }) => {
	const catalog = useBackendCatalog();
	const pair = `${domain}/${backendName}` as SectionKey;
	const backend = catalog.byID.get(pair);
	const toast = useToast();
	const { t } = useTranslation();
	const [reveal, setReveal] = useState(false);
	const [editorTarget, setEditorTarget] = useState<ProfileEditorTarget | null>(null);
	const [profileToRemove, setProfileToRemove] = useState<string | null>(null);
	const [removing, setRemoving] = useState(false);

	const swrKey = backend ? sectionKey(backend.domain, backend.name, reveal) : null;
	const { data, error, isLoading, mutate } = useSWR(swrKey, () => {
		if (!backend) return Promise.reject(new Error("unknown section"));
		return getSection(backend.domain, backend.name, reveal);
	});

	if (catalog.error) {
		return (
			<Alert variant="destructive">
				<AlertTitle>{t("settings.loadFailedTitle")}</AlertTitle>
				<AlertDescription>{catalog.error.message}</AlertDescription>
			</Alert>
		);
	}
	if (catalog.isLoading) {
		return (
			<div className="flex items-center gap-2 text-sm text-muted-foreground">
				<Spinner /> {t("detail.loading")}
			</div>
		);
	}
	if (!backend || !backend.profile.configurable) {
		return (
			<Alert variant="destructive">
				<AlertTitle>{t("detail.unknownSectionTitle")}</AlertTitle>
				<AlertDescription>
					{t("detail.unknownSectionBody", {
						domain,
						backend: backendName,
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
		setRemoving(true);
		try {
			await removeProfile(selectedBackend.domain, selectedBackend.name, name);
			toast.success(t("toast.removed", { name }));
			setProfileToRemove(null);
			void refresh();
		} catch (err) {
			const e = err as { code?: string; message: string };
			toast.error(e.message, { description: e.code });
		} finally {
			setRemoving(false);
		}
	}

	const profiles = data?.section.profiles ?? {};
	const defaultName = data?.section.default ?? "";
	const profileNames = Object.keys(profiles).sort();
	const title = backendTitle(t, backend);
	const description = backendDescription(t, backend);

	return (
		<div className={embedded ? "space-y-3" : "space-y-5"}>
			<div className="flex flex-wrap items-end justify-between gap-3 border-b border-border pb-4">
				<div className="min-w-0">
					<div className="flex items-center gap-2">
						<h1
							className={
								embedded
									? "text-lg font-semibold tracking-tight"
									: "text-[28px] font-medium tracking-tight"
							}
						>
							{title}
						</h1>
						<Badge variant="outline" className="font-mono text-[10px]">
							{backend.id}
						</Badge>
					</div>
					<p className="mt-1 text-xs text-muted-foreground">{description}</p>
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

			<AlertDialog
				open={profileToRemove !== null}
				onOpenChange={(open) => {
					if (!open && !removing) setProfileToRemove(null);
				}}
			>
				<AlertDialogContent>
					<AlertDialogHeader>
						<AlertDialogTitle>{t("detail.remove")}</AlertDialogTitle>
						<AlertDialogDescription>
							{profileToRemove ? t("detail.confirmRemove", { name: profileToRemove }) : ""}
						</AlertDialogDescription>
					</AlertDialogHeader>
					<AlertDialogFooter>
						<AlertDialogCancel disabled={removing}>{t("form.cancel")}</AlertDialogCancel>
						<AlertDialogAction
							variant="destructive"
							disabled={removing}
							onClick={(event) => {
								event.preventDefault();
								if (profileToRemove) void onRemove(profileToRemove);
							}}
						>
							{t("detail.remove")}
						</AlertDialogAction>
					</AlertDialogFooter>
				</AlertDialogContent>
			</AlertDialog>

			<div className="space-y-3">
				{isLoading ? (
					<div className="flex items-center gap-2 text-sm text-muted-foreground">
						<Spinner /> {t("detail.loading")}
					</div>
				) : null}
				{!isLoading && profileNames.length === 0 ? (
					<Empty className="min-h-40 border border-dashed border-border">
						<EmptyHeader>
							<EmptyDescription>{t("detail.empty")}</EmptyDescription>
						</EmptyHeader>
					</Empty>
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
																<Badge className="border-success-border bg-success-surface text-success-foreground">
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
																variant="ghost"
																className="text-error-foreground hover:bg-error-surface hover:text-error-foreground"
																onClick={() => setProfileToRemove(name)}
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
