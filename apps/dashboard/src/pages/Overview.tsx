// Overview renders the workspace `one serve` was launched in: identity,
// environment list, and a card per sub-project with badges for any missing
// configuration domains (container / deploy / env). When the workspace
// itself is missing a workspace-scoped backend (env), that surfaces as a
// top-of-page alert so we don't duplicate it under every project card.
// Dev command is deliberately not surfaced — `one add` derives it from
// package.json scripts, and an empty value is a valid "this project does
// not participate in `one dev`" signal, not a missing-config issue.
//
// Every missing-config badge is clickable: backend issues open
// MissingConfigDialog, while profile issues open the credential form in-place.
// Successful manifest/profile changes refresh the SWR cache so badges flip
// green without a reload.

import { AlertTriangle, ArrowRight, FolderTree, Layers, Package } from "lucide-react";
import type React from "react";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router-dom";
import { mutate } from "swr";
import { type ProfileByPair, upsertProfile } from "@/api/configure";
import { overviewKey } from "@/api/workspace";
import { MissingConfigDialog } from "@/components/MissingConfigDialog";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent } from "@/components/ui/dialog";
import { useToast } from "@/hooks/useToast";
import { type AnyProfile, ProfileForm, emptyProfile } from "@/pages/SectionDetail";
import type {
	Overview as OverviewPayload,
	OverviewIssue,
	OverviewIssueDomain,
	OverviewProjectKind,
	SectionKey,
} from "@/types/api";
import { SECTION_KEYS } from "@/types/api";

const KIND_ICON: Record<OverviewProjectKind, React.ComponentType<{ className?: string }>> = {
	app: Layers,
	service: FolderTree,
	package: Package,
};

// Issue domains we surface as badges, in the order users care about most.
const ISSUE_DOMAIN_ORDER: OverviewIssueDomain[] = ["deploy", "container", "env"];

function sortIssues(issues: OverviewIssue[] | undefined): OverviewIssue[] {
	if (!issues) return [];
	const rank = new Map(ISSUE_DOMAIN_ORDER.map((d, i) => [d, i]));
	return [...issues].sort((a, b) => (rank.get(a.domain) ?? 99) - (rank.get(b.domain) ?? 99));
}

function issueKey(issue: OverviewIssue): string {
	return [issue.domain, issue.reason ?? "backend", issue.backend ?? "", issue.profile ?? ""].join(
		":",
	);
}

// FixTarget is what the dialog needs to know: which domain, and (for
// per-project issues) which project. env is workspace-scoped, so it has no
// project.
interface FixTarget {
	domain: OverviewIssueDomain;
	projectName?: string;
}

interface ProfileFixTarget {
	sectionKey: SectionKey;
	name: string;
	mode: "add" | "edit";
}

function isSectionKey(value: string | undefined): value is SectionKey {
	return Boolean(value && SECTION_KEYS.includes(value as SectionKey));
}

function profileIssueSection(issue: OverviewIssue): SectionKey | null {
	if (isSectionKey(issue.section)) return issue.section;
	const fallback = issue.backend ? `${issue.domain}/${issue.backend}` : undefined;
	if (isSectionKey(fallback)) return fallback;
	return null;
}

function profileIssueLabel(issue: OverviewIssue): string {
	return profileIssueSection(issue) ?? issue.section ?? `${issue.domain}/${issue.backend ?? ""}`;
}

export const Overview: React.FC<{ data: OverviewPayload }> = ({ data }) => {
	const { t } = useTranslation();
	const toast = useToast();
	const ws = data.workspace;
	const workspaceIssues = sortIssues(data.issues);
	const projects = data.projects ?? [];

	const [fixTarget, setFixTarget] = useState<FixTarget | null>(null);
	const [profileFixTarget, setProfileFixTarget] = useState<ProfileFixTarget | null>(null);
	const [profileDialogSnapshot, setProfileDialogSnapshot] = useState<ProfileFixTarget | null>(null);

	useEffect(() => {
		if (profileFixTarget) setProfileDialogSnapshot(profileFixTarget);
	}, [profileFixTarget]);

	// The mutate endpoints return the rebuilt Overview; push it straight
	// into the SWR cache (no revalidate) so the page repaints instantly.
	const handleUpdated = (next: OverviewPayload) => {
		void mutate(overviewKey, next, { revalidate: false });
	};

	const issueText = (issue: OverviewIssue) =>
		issue.reason === "profile"
			? t("overview.issue.missingProfile", {
					section: issue.section ?? `${issue.domain}/${issue.backend ?? ""}`,
					profile: issue.profile ? ` "${issue.profile}"` : "",
					defaultValue: issue.message,
				})
			: t(`overview.issue.${issue.severity}.${issue.domain}`, {
					defaultValue: issue.message,
				});

	const issueBadgeLabel = (issue: OverviewIssue) =>
		issue.reason === "profile"
			? profileIssueLabel(issue)
			: t(`overview.issue.label.${issue.domain}`, { defaultValue: issue.domain });

	const openProfileFix = (issue: OverviewIssue) => {
		const sectionKey = profileIssueSection(issue);
		if (!sectionKey) return;
		setProfileFixTarget({
			sectionKey,
			name: issue.profile || "default",
			mode: issue.profile ? "edit" : "add",
		});
	};

	async function handleProfileSubmit(name: string, profile: AnyProfile, use: boolean) {
		if (!profileFixTarget) return;
		const [domain, backend] = profileFixTarget.sectionKey.split("/");
		try {
			const res = await upsertProfile(domain, backend, {
				name,
				profile: profile as ProfileByPair[SectionKey],
				use,
			});
			toast.success(
				res.status === "updated" ? t("toast.updated", { name }) : t("toast.created", { name }),
				{ description: res.default ? t("toast.setDefaultAfterSaveHint") : undefined },
			);
			setProfileFixTarget(null);
			void mutate(overviewKey);
		} catch (err) {
			const e = err as { code?: string; message: string };
			toast.error(e.message, { description: e.code });
		}
	}

	return (
		<div className="space-y-6">
			<header className="space-y-2">
				<div className="flex items-baseline gap-2">
					<h1 className="text-xl font-semibold tracking-tight">
						{ws?.name || t("overview.untitledWorkspace")}
					</h1>
					{ws?.id ? (
						<Badge variant="outline" className="font-mono">
							{ws.id}
						</Badge>
					) : null}
				</div>
				{data.root ? (
					<p className="text-xs font-mono text-muted-foreground truncate">{data.root}</p>
				) : null}
				{ws?.environments && ws.environments.length > 0 ? (
					<div className="flex items-center gap-2 text-xs text-muted-foreground">
						<span>{t("overview.environments")}</span>
						<span className="flex flex-wrap gap-1">
							{ws.environments.map((name) => (
								<Badge
									key={name}
									variant={name === ws.defaultEnvironment ? "default" : "secondary"}
								>
									{name}
								</Badge>
							))}
						</span>
					</div>
				) : null}
			</header>

			{workspaceIssues.length > 0 ? (
				<Alert variant="destructive">
					<AlertTriangle className="h-4 w-4" />
					<AlertTitle>{t("overview.workspaceIssuesTitle")}</AlertTitle>
					<AlertDescription>
						<ul className="mt-1 space-y-1">
							{workspaceIssues.map((iss) => (
								<li key={issueKey(iss)} className="flex items-center gap-2">
									<span>{issueText(iss)}</span>
									{iss.reason === "profile" && profileIssueSection(iss) ? (
										<button
											type="button"
											onClick={() => openProfileFix(iss)}
											className="rounded border border-current/30 px-1.5 py-0.5 text-[11px] font-medium hover:bg-current/10"
										>
											{t("overview.fix.profileCta")}
										</button>
									) : (
										<button
											type="button"
											onClick={() => setFixTarget({ domain: iss.domain })}
											className="rounded border border-current/30 px-1.5 py-0.5 text-[11px] font-medium hover:bg-current/10"
										>
											{t("overview.fix.cta")}
										</button>
									)}
								</li>
							))}
						</ul>
					</AlertDescription>
				</Alert>
			) : null}

			<section className="space-y-3">
				<div className="flex items-baseline gap-2 border-b border-border/60 pb-1.5">
					<h2 className="text-sm font-semibold tracking-wide uppercase text-muted-foreground">
						{t("overview.projects")}
					</h2>
					<span className="text-[11px] text-muted-foreground/60 font-mono">{projects.length}</span>
				</div>
				{projects.length === 0 ? (
					<p className="text-sm text-muted-foreground">{t("overview.empty")}</p>
				) : (
					<div className="grid gap-3 md:grid-cols-2 lg:grid-cols-3">
						{projects.map((p) => {
							const Icon = KIND_ICON[p.kind] ?? Layers;
							const issues = sortIssues(p.issues);
							return (
								<Card key={p.name} className="h-full">
									<CardHeader>
										<div className="flex items-start justify-between gap-2">
											<div className="space-y-1 min-w-0">
												<CardTitle className="flex items-center gap-2">
													<Icon className="h-4 w-4 text-primary shrink-0" />
													<span className="truncate">{p.name}</span>
												</CardTitle>
												<p className="text-[11px] font-mono text-muted-foreground truncate">
													{p.relativeDir}
												</p>
												<CardDescription className="flex flex-wrap items-center gap-1">
													<Badge variant="outline">
														{t(`overview.kinds.${p.kind}`, { defaultValue: p.kind })}
													</Badge>
													{p.toolchain ? (
														<Badge variant="secondary" className="font-mono">
															{p.toolchain}
														</Badge>
													) : null}
													{p.templateId ? (
														<Badge variant="secondary" className="font-mono">
															{p.templateId}
														</Badge>
													) : null}
												</CardDescription>
											</div>
										</div>
									</CardHeader>
									<CardContent className="space-y-2 text-xs">
										{p.domains && Object.keys(p.domains).length > 0 ? (
											<div className="flex flex-wrap gap-1">
												{Object.entries(p.domains).map(([dom, kind]) => (
													<Badge key={dom} variant="outline" className="font-mono">
														{dom}: {kind}
													</Badge>
												))}
											</div>
										) : null}
										{issues.length > 0 ? (
											<div className="flex flex-wrap gap-1">
												{issues.map((iss) =>
													iss.reason === "profile" && profileIssueSection(iss) ? (
														<button
															key={issueKey(iss)}
															type="button"
															onClick={() => openProfileFix(iss)}
															title={issueText(iss)}
															className="rounded-full focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
														>
															<Badge
																variant="destructive"
																className="cursor-pointer hover:opacity-90"
															>
																<AlertTriangle className="h-3 w-3" />
																{issueBadgeLabel(iss)}
															</Badge>
														</button>
													) : (
														<button
															key={issueKey(iss)}
															type="button"
															onClick={() =>
																setFixTarget({
																	domain: iss.domain,
																	projectName: p.name,
																})
															}
															title={issueText(iss)}
															className="rounded-full focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
														>
															<Badge
																variant="destructive"
																className="cursor-pointer hover:opacity-90"
															>
																<AlertTriangle className="h-3 w-3" />
																{issueBadgeLabel(iss)}
															</Badge>
														</button>
													),
												)}
											</div>
										) : (
											<span className="text-emerald-600 dark:text-emerald-400">
												{t("overview.allGood")}
											</span>
										)}
									</CardContent>
								</Card>
							);
						})}
					</div>
				)}
			</section>

			<div className="pt-2">
				<Link
					to="/profile"
					className="inline-flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground"
				>
					{t("overview.manageProfilesCta")}
					<ArrowRight className="h-3 w-3" />
				</Link>
			</div>

			<MissingConfigDialog
				domain={fixTarget?.domain ?? null}
				projectName={fixTarget?.projectName}
				open={fixTarget !== null}
				onOpenChange={(next) => {
					if (!next) setFixTarget(null);
				}}
				onUpdated={handleUpdated}
			/>

			<Dialog
				open={profileFixTarget !== null}
				onOpenChange={(next) => {
					if (!next) setProfileFixTarget(null);
				}}
			>
				<DialogContent>
					{profileDialogSnapshot ? (
						<ProfileForm
							sectionKey={profileDialogSnapshot.sectionKey}
							initialName={profileDialogSnapshot.name}
							initialProfile={emptyProfile(profileDialogSnapshot.sectionKey)}
							mode={profileDialogSnapshot.mode}
							hasDefault={profileDialogSnapshot.mode === "edit"}
							onCancel={() => setProfileFixTarget(null)}
							onSubmit={handleProfileSubmit}
						/>
					) : null}
				</DialogContent>
			</Dialog>
		</div>
	);
};
