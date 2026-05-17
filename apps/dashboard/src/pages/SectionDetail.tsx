// SectionDetail renders one (domain, backend) page: list profiles + add /
// edit form. Each backend has its own form fields; the form component
// switches on `key` to render the right inputs.

import { Check, Eye, EyeOff, Loader2, Plus, Save, Star, Trash2 } from "lucide-react";
import type React from "react";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { useParams } from "react-router-dom";
import useSWR from "swr";
import {
	type ProfileByPair,
	getSection,
	removeProfile,
	sectionKey,
	setDefault,
	upsertProfile,
} from "@/api/configure";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "@/components/ui/table";
import { useToast } from "@/hooks/useToast";
import { SECTION_META, type SectionKey, type SectionMeta, SECTION_KEYS } from "@/types/api";

export type AnyProfile = ProfileByPair[keyof ProfileByPair];

function isKnownSection(domain: string, backend: string): SectionMeta | null {
	const key = `${domain}/${backend}`;
	if (!SECTION_KEYS.includes(key as SectionKey)) return null;
	return SECTION_META[key as SectionKey];
}

export const SectionDetail: React.FC = () => {
	const params = useParams<{ domain: string; backend: string }>();
	const meta = isKnownSection(params.domain ?? "", params.backend ?? "");
	const toast = useToast();
	const { t } = useTranslation();
	const [reveal, setReveal] = useState(false);
	const [editing, setEditing] = useState<{ name: string; profile: AnyProfile } | null>(null);
	const [adding, setAdding] = useState(false);

	// 关闭时保留最后一次的内容快照，避免子节点先消失导致弹窗高度坍缩、再 fade-out
	type DialogSnapshot =
		| { kind: "add" }
		| { kind: "edit"; name: string; profile: AnyProfile }
		| null;
	const [dialogSnapshot, setDialogSnapshot] = useState<DialogSnapshot>(null);
	useEffect(() => {
		if (adding) setDialogSnapshot({ kind: "add" });
		else if (editing)
			setDialogSnapshot({ kind: "edit", name: editing.name, profile: editing.profile });
	}, [adding, editing]);

	const swrKey = meta ? sectionKey(meta.domain, meta.backend, reveal) : null;
	const { data, error, isLoading, mutate } = useSWR(swrKey, () => {
		if (!meta) return Promise.reject(new Error("unknown section"));
		return getSection(meta.domain, meta.backend, reveal);
	});

	if (!meta) {
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

	const refresh = () => mutate();

	async function onSubmit(name: string, profile: AnyProfile, use: boolean) {
		try {
			const res = await upsertProfile(meta!.domain, meta!.backend, {
				name,
				profile: profile as ProfileByPair[SectionKey],
				use,
			});
			toast.success(
				res.status === "updated" ? t("toast.updated", { name }) : t("toast.created", { name }),
				{ description: res.default ? t("toast.setDefaultAfterSaveHint") : undefined },
			);
			setAdding(false);
			setEditing(null);
			refresh();
		} catch (err) {
			const e = err as { code?: string; message: string };
			toast.error(e.message, { description: e.code });
		}
	}

	async function onUse(name: string) {
		try {
			await setDefault(meta!.domain, meta!.backend, name);
			toast.success(t("toast.setDefault", { name }));
			refresh();
		} catch (err) {
			const e = err as { code?: string; message: string };
			toast.error(e.message, { description: e.code });
		}
	}

	async function onRemove(name: string) {
		if (!window.confirm(t("detail.confirmRemove", { name }))) return;
		try {
			await removeProfile(meta!.domain, meta!.backend, name);
			toast.success(t("toast.removed", { name }));
			refresh();
		} catch (err) {
			const e = err as { code?: string; message: string };
			toast.error(e.message, { description: e.code });
		}
	}

	const profiles = data?.section.profiles ?? {};
	const defaultName = data?.section.default ?? "";
	const profileNames = Object.keys(profiles).sort();
	const title = t(`sections.${meta.domain}.${meta.backend}.title`, {
		defaultValue: meta.title,
	});
	const description = t(`sections.${meta.domain}.${meta.backend}.description`, {
		defaultValue: meta.description,
	});

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
						onClick={() => setReveal((v) => !v)}
						title={reveal ? t("detail.hideSecrets") : t("detail.showSecrets")}
					>
						{reveal ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
						{reveal ? t("detail.showingSecrets") : t("detail.showSecrets")}
					</Button>
					{!adding && !editing ? (
						<Button size="sm" onClick={() => setAdding(true)}>
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

			<Dialog
				open={adding || editing !== null}
				onOpenChange={(open) => {
					if (!open) {
						setAdding(false);
						setEditing(null);
					}
				}}
			>
				<DialogContent>
					{dialogSnapshot?.kind === "add" ? (
						<ProfileForm
							sectionKey={meta.key}
							initialName=""
							initialProfile={emptyProfile(meta.key)}
							onCancel={() => setAdding(false)}
							onSubmit={onSubmit}
							mode="add"
							hasDefault={defaultName !== ""}
						/>
					) : null}
					{dialogSnapshot?.kind === "edit" ? (
						<ProfileForm
							sectionKey={meta.key}
							initialName={dialogSnapshot.name}
							initialProfile={dialogSnapshot.profile}
							onCancel={() => setEditing(null)}
							onSubmit={onSubmit}
							mode="edit"
							hasDefault={defaultName !== ""}
						/>
					) : null}
				</DialogContent>
			</Dialog>

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
											const p = profiles[name] as AnyProfile;
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
														<ProfileSummary sectionKey={meta.key} profile={p} />
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
																onClick={() => setEditing({ name, profile: p })}
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

// ──────────────────────────── form ──────────────────────────────────────

export function emptyProfile(key: SectionKey): AnyProfile {
	switch (key) {
		case "env/infisical":
			return {
				siteUrl: "https://app.infisical.com",
				credentials: { clientId: "", clientSecret: "" },
			};
		case "deploy/aliyun-oss":
		case "deploy/tencent-cos":
		case "deploy/aws-s3":
		case "deploy/r2":
			return {
				endpoint: "",
				region: "",
				forcePathStyle: false,
				credentials: { accessKeyId: "", accessKeySecret: "" },
			};
		case "deploy/minio":
		case "deploy/rustfs":
			return {
				endpoint: "",
				region: "us-east-1",
				forcePathStyle: true,
				credentials: { accessKeyId: "", accessKeySecret: "" },
			};
		case "deploy/kustomize":
			return { kubeconfigPath: "", kubeconfigContext: "" };
		case "deploy/vercel":
			return { team: "", credentials: { apiToken: "" } };
		case "deploy/cloudflare":
			return { accountId: "", credentials: { apiToken: "" } };
		case "deploy/edgeone":
			return { region: "", credentials: { apiToken: "" } };
		case "container/docker":
			return {
				registry: "",
				namespace: "",
				credentials: { username: "", password: "" },
			};
		case "container/dockerhub":
		case "container/ghcr":
			return {
				namespace: "",
				credentials: { username: "", password: "" },
			};
		case "container/acr":
			return {
				region: "",
				namespace: "",
				credentials: { username: "", password: "" },
			};
	}
}

interface ProfileFormProps {
	sectionKey: SectionKey;
	initialName: string;
	initialProfile: AnyProfile;
	mode: "add" | "edit";
	hasDefault: boolean;
	onCancel(): void;
	onSubmit(name: string, profile: AnyProfile, use: boolean): Promise<void>;
}

export const ProfileForm: React.FC<ProfileFormProps> = ({
	sectionKey,
	initialName,
	initialProfile,
	mode,
	hasDefault,
	onCancel,
	onSubmit,
}) => {
	const { t } = useTranslation();
	const [name, setName] = useState(initialName);
	const [profile, setProfile] = useState<AnyProfile>(initialProfile);
	const [use, setUse] = useState(false);
	const [submitting, setSubmitting] = useState(false);

	async function handleSubmit(e: React.FormEvent) {
		e.preventDefault();
		setSubmitting(true);
		try {
			await onSubmit(name.trim(), profile, use);
		} finally {
			setSubmitting(false);
		}
	}

	return (
		<>
			<DialogHeader>
				<DialogTitle>
					{mode === "add" ? t("form.addTitle") : t("form.editTitle", { name: initialName })}
				</DialogTitle>
				<DialogDescription>
					{mode === "add" ? t("form.addDescription") : t("form.editDescription")}
				</DialogDescription>
			</DialogHeader>
			<form onSubmit={handleSubmit} className="space-y-3">
				<FieldRow>
					<Label htmlFor="profile-name">{t("form.profileName")}</Label>
					<Input
						id="profile-name"
						value={name}
						onChange={(e) => setName(e.target.value)}
						placeholder={t("form.profileNamePlaceholder")}
						disabled={mode === "edit"}
						required
					/>
				</FieldRow>

				<BackendFields sectionKey={sectionKey} profile={profile} setProfile={setProfile} />

				<FieldRow>
					<label className="flex items-center gap-2 text-sm">
						<input type="checkbox" checked={use} onChange={(e) => setUse(e.target.checked)} />
						<span>
							{hasDefault ? t("form.setDefaultAfterSave") : t("form.setDefaultAfterSaveAuto")}
						</span>
					</label>
				</FieldRow>

				<div className="flex items-center justify-end gap-2 pt-2">
					<Button type="button" variant="outline" onClick={onCancel}>
						{t("form.cancel")}
					</Button>
					<Button type="submit" disabled={submitting || !name.trim()}>
						{submitting ? (
							<Loader2 className="h-4 w-4 animate-spin" />
						) : (
							<Save className="h-4 w-4" />
						)}
						{t("form.save")}
					</Button>
				</div>
			</form>
		</>
	);
};

const FieldRow: React.FC<React.PropsWithChildren> = ({ children }) => (
	<div className="grid gap-2">{children}</div>
);

interface BackendFieldsProps {
	sectionKey: SectionKey;
	profile: AnyProfile;
	setProfile(p: AnyProfile): void;
}

const BackendFields: React.FC<BackendFieldsProps> = ({ sectionKey, profile, setProfile }) => {
	const { t } = useTranslation();
	switch (sectionKey) {
		case "env/infisical": {
			const p = profile as ProfileByPair["env/infisical"];
			return (
				<>
					<FieldRow>
						<Label>{t("form.fields.siteUrl")}</Label>
						<Input
							value={p.siteUrl ?? ""}
							onChange={(e) => setProfile({ ...p, siteUrl: e.target.value })}
							placeholder="https://app.infisical.com"
						/>
					</FieldRow>
					<FieldRow>
						<Label>{t("form.fields.clientId")}</Label>
						<Input
							value={p.credentials?.clientId ?? ""}
							onChange={(e) =>
								setProfile({
									...p,
									credentials: {
										clientId: e.target.value,
										clientSecret: p.credentials?.clientSecret ?? "",
									},
								})
							}
						/>
					</FieldRow>
					<FieldRow>
						<Label>{t("form.fields.clientSecret")}</Label>
						<Input
							type="password"
							value={p.credentials?.clientSecret ?? ""}
							onChange={(e) =>
								setProfile({
									...p,
									credentials: {
										clientId: p.credentials?.clientId ?? "",
										clientSecret: e.target.value,
									},
								})
							}
						/>
					</FieldRow>
				</>
			);
		}
		case "deploy/aliyun-oss":
		case "deploy/tencent-cos":
		case "deploy/aws-s3":
		case "deploy/minio":
		case "deploy/rustfs":
		case "deploy/r2": {
			const p = profile as ProfileByPair["deploy/aws-s3"];
			return (
				<>
					<FieldRow>
						<Label>{t("form.fields.endpoint")}</Label>
						<Input
							value={p.endpoint ?? ""}
							onChange={(e) => setProfile({ ...p, endpoint: e.target.value })}
							placeholder={t("form.fields.endpointPlaceholder")}
						/>
					</FieldRow>
					<FieldRow>
						<Label>{t("form.fields.region")}</Label>
						<Input
							value={p.region ?? ""}
							onChange={(e) => setProfile({ ...p, region: e.target.value })}
							placeholder={t("form.fields.regionPlaceholder")}
						/>
					</FieldRow>
					<FieldRow>
						<label className="flex items-center gap-2 text-sm">
							<input
								type="checkbox"
								checked={!!p.forcePathStyle}
								onChange={(e) => setProfile({ ...p, forcePathStyle: e.target.checked })}
							/>
							<span>{t("form.fields.forcePathStyle")}</span>
						</label>
					</FieldRow>
					<FieldRow>
						<Label>{t("form.fields.accessKeyId")}</Label>
						<Input
							value={p.credentials?.accessKeyId ?? ""}
							onChange={(e) =>
								setProfile({
									...p,
									credentials: {
										accessKeyId: e.target.value,
										accessKeySecret: p.credentials?.accessKeySecret ?? "",
									},
								})
							}
						/>
					</FieldRow>
					<FieldRow>
						<Label>{t("form.fields.accessKeySecret")}</Label>
						<Input
							type="password"
							value={p.credentials?.accessKeySecret ?? ""}
							onChange={(e) =>
								setProfile({
									...p,
									credentials: {
										accessKeyId: p.credentials?.accessKeyId ?? "",
										accessKeySecret: e.target.value,
									},
								})
							}
						/>
					</FieldRow>
				</>
			);
		}
		case "deploy/kustomize": {
			const p = profile as ProfileByPair["deploy/kustomize"];
			return (
				<>
					<FieldRow>
						<Label>{t("form.fields.kubeconfigPath")}</Label>
						<Input
							value={p.kubeconfigPath ?? ""}
							onChange={(e) => setProfile({ ...p, kubeconfigPath: e.target.value })}
							placeholder="~/.kube/config"
						/>
					</FieldRow>
					<FieldRow>
						<Label>{t("form.fields.kubeconfigContext")}</Label>
						<Input
							value={p.kubeconfigContext ?? ""}
							onChange={(e) => setProfile({ ...p, kubeconfigContext: e.target.value })}
							placeholder={t("form.fields.kubeconfigContextPlaceholder")}
						/>
					</FieldRow>
				</>
			);
		}
		case "deploy/vercel": {
			const p = profile as ProfileByPair["deploy/vercel"];
			return (
				<>
					<FieldRow>
						<Label>{t("form.fields.teamSlug")}</Label>
						<Input
							value={p.team ?? ""}
							onChange={(e) => setProfile({ ...p, team: e.target.value })}
							placeholder={t("form.fields.teamSlugPlaceholder")}
						/>
					</FieldRow>
					<FieldRow>
						<Label>{t("form.fields.apiToken")}</Label>
						<Input
							type="password"
							value={p.credentials?.apiToken ?? ""}
							onChange={(e) =>
								setProfile({
									...p,
									credentials: { apiToken: e.target.value },
								})
							}
						/>
					</FieldRow>
				</>
			);
		}
		case "deploy/cloudflare": {
			const p = profile as ProfileByPair["deploy/cloudflare"];
			return (
				<>
					<FieldRow>
						<Label>{t("form.fields.accountId")}</Label>
						<Input
							value={p.accountId ?? ""}
							onChange={(e) => setProfile({ ...p, accountId: e.target.value })}
							placeholder={t("form.fields.accountIdPlaceholder")}
						/>
					</FieldRow>
					<FieldRow>
						<Label>{t("form.fields.apiToken")}</Label>
						<Input
							type="password"
							value={p.credentials?.apiToken ?? ""}
							onChange={(e) =>
								setProfile({
									...p,
									credentials: { apiToken: e.target.value },
								})
							}
						/>
					</FieldRow>
				</>
			);
		}
		case "deploy/edgeone": {
			const p = profile as ProfileByPair["deploy/edgeone"];
			return (
				<>
					<FieldRow>
						<Label>{t("form.fields.regionEdgeOne")}</Label>
						<Input
							value={p.region ?? ""}
							onChange={(e) => setProfile({ ...p, region: e.target.value })}
							placeholder={t("form.fields.regionEdgeOnePlaceholder")}
						/>
					</FieldRow>
					<FieldRow>
						<Label>{t("form.fields.apiToken")}</Label>
						<Input
							type="password"
							value={p.credentials?.apiToken ?? ""}
							onChange={(e) =>
								setProfile({
									...p,
									credentials: { apiToken: e.target.value },
								})
							}
						/>
					</FieldRow>
				</>
			);
		}
		case "container/docker":
		case "container/dockerhub":
		case "container/ghcr":
		case "container/acr": {
			const p = profile as ProfileByPair["container/docker"];
			return (
				<>
					{sectionKey === "container/docker" && (
						<FieldRow>
							<Label>{t("form.fields.registry")}</Label>
							<Input
								value={p.registry ?? ""}
								onChange={(e) => setProfile({ ...p, registry: e.target.value })}
								placeholder={t("form.fields.registryPlaceholder")}
							/>
						</FieldRow>
					)}
					{sectionKey === "container/acr" && (
						<FieldRow>
							<Label>{t("form.fields.acrRegion")}</Label>
							<Input
								value={p.region ?? ""}
								onChange={(e) => setProfile({ ...p, region: e.target.value })}
								placeholder={t("form.fields.acrRegionPlaceholder")}
							/>
						</FieldRow>
					)}
					<FieldRow>
						<Label>{t("form.fields.namespace")}</Label>
						<Input
							value={p.namespace ?? ""}
							onChange={(e) => setProfile({ ...p, namespace: e.target.value })}
							placeholder={t("form.fields.namespacePlaceholder")}
						/>
					</FieldRow>
					<FieldRow>
						<Label>{t("form.fields.username")}</Label>
						<Input
							value={p.credentials?.username ?? ""}
							onChange={(e) =>
								setProfile({
									...p,
									credentials: {
										username: e.target.value,
										password: p.credentials?.password ?? "",
									},
								})
							}
						/>
					</FieldRow>
					<FieldRow>
						<Label>{t("form.fields.password")}</Label>
						<Input
							type="password"
							value={p.credentials?.password ?? ""}
							onChange={(e) =>
								setProfile({
									...p,
									credentials: {
										username: p.credentials?.username ?? "",
										password: e.target.value,
									},
								})
							}
						/>
					</FieldRow>
				</>
			);
		}
	}
};

interface ProfileSummaryProps {
	sectionKey: SectionKey;
	profile: AnyProfile;
}

const ProfileSummary: React.FC<ProfileSummaryProps> = ({ sectionKey, profile }) => {
	const { t } = useTranslation();
	switch (sectionKey) {
		case "env/infisical": {
			const p = profile as ProfileByPair["env/infisical"];
			return (
				<span className="text-xs">
					{t("form.summary.site", { site: p.siteUrl || t("form.summary.notSet") })}
				</span>
			);
		}
		case "deploy/aliyun-oss":
		case "deploy/tencent-cos":
		case "deploy/aws-s3":
		case "deploy/minio":
		case "deploy/rustfs":
		case "deploy/r2": {
			const p = profile as ProfileByPair["deploy/aws-s3"];
			return (
				<span className="text-xs">
					{t("form.summary.endpointRegion", {
						endpoint: p.endpoint || t("form.summary.endpointDefault"),
						region: p.region || t("form.summary.regionDefault"),
					})}
				</span>
			);
		}
		case "deploy/kustomize": {
			const p = profile as ProfileByPair["deploy/kustomize"];
			return (
				<span className="text-xs">
					{t("form.summary.contextFile", {
						context: p.kubeconfigContext || t("form.summary.contextDefault"),
						file: p.kubeconfigPath || t("form.summary.kubeconfigDefault"),
					})}
				</span>
			);
		}
		case "deploy/vercel": {
			const p = profile as ProfileByPair["deploy/vercel"];
			return (
				<span className="text-xs">
					{t("form.summary.team", { team: p.team || t("form.summary.teamPersonal") })}
				</span>
			);
		}
		case "deploy/cloudflare": {
			const p = profile as ProfileByPair["deploy/cloudflare"];
			return (
				<span className="text-xs">
					{t("form.summary.account", {
						account: p.accountId || t("form.summary.accountTokenDefault"),
					})}
				</span>
			);
		}
		case "deploy/edgeone": {
			const p = profile as ProfileByPair["deploy/edgeone"];
			return (
				<span className="text-xs">
					{t("form.summary.regionLabel", {
						region: p.region || t("form.summary.regionDefaultLabel"),
					})}
				</span>
			);
		}
		case "container/docker": {
			const p = profile as ProfileByPair["container/docker"];
			return (
				<span className="text-xs">
					{p.registry || t("form.summary.registryUnset")}
					{p.namespace ? ` / ${p.namespace}` : ""}
				</span>
			);
		}
		case "container/dockerhub": {
			const p = profile as ProfileByPair["container/dockerhub"];
			return (
				<span className="text-xs">
					index.docker.io
					{p.namespace ? ` / ${p.namespace}` : ""}
				</span>
			);
		}
		case "container/ghcr": {
			const p = profile as ProfileByPair["container/ghcr"];
			return (
				<span className="text-xs">
					ghcr.io
					{p.namespace ? ` / ${p.namespace}` : ""}
				</span>
			);
		}
		case "container/acr": {
			const p = profile as ProfileByPair["container/acr"];
			return (
				<span className="text-xs">
					{p.region ? `registry.${p.region}.aliyuncs.com` : t("form.summary.acrRegionUnset")}
					{p.namespace ? ` / ${p.namespace}` : ""}
				</span>
			);
		}
	}
};
