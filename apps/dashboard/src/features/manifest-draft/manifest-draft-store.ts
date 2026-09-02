import { createStore } from "@/lib/utils";
import type { ProjectManifestPatch, ProfileValue, WorkspaceManifestPatch } from "@/types/api";

export type ManifestDraftSection = "general" | "environment" | "container" | "deploy";

type DraftValue = string | boolean | number | null | undefined;

export interface ManifestDraftSummary {
	id: string;
	project: string;
	section: ManifestDraftSection;
	path: string;
	labelKey: string;
	before: DraftValue;
	after: DraftValue;
	changed: boolean;
}

export interface WorkspaceManifestDraft {
	revision: string;
	workspace?: WorkspaceManifestPatch;
	changes: Record<string, ProjectManifestPatch>;
	summaries: ManifestDraftSummary[];
}

export const WORKSPACE_DRAFT_SUBJECT = "__workspace__";

interface StageSectionInput {
	entryId?: string;
	revision: string;
	project: string;
	section: ManifestDraftSection;
	initial: object;
	next: object;
	labels: Record<string, string>;
}

interface StageWorkspaceSectionInput {
	entryId?: string;
	revision: string;
	section: "environment";
	initial: object;
	next: object;
	labels: Record<string, string>;
}

interface ManifestDraftState {
	drafts: Readonly<Record<string, WorkspaceManifestDraft>>;
	stageSection(input: StageSectionInput): void;
	stageWorkspaceSection(input: StageWorkspaceSectionInput): void;
	commitWorkspaceSection(
		entryId: string | undefined,
		section: "environment",
		revision: string,
	): void;
	clearWorkspace(entryId?: string): void;
}

export function manifestDraftKey(entryId?: string): string {
	return entryId?.trim() || "__current__";
}

function flatten(prefix: string, value: unknown, output: Record<string, DraftValue>) {
	if (value !== null && typeof value === "object" && !Array.isArray(value)) {
		for (const [key, child] of Object.entries(value as Record<string, unknown>)) {
			flatten(prefix ? `${prefix}/${key}` : key, child, output);
		}
		return;
	}
	output[prefix] = value as DraftValue;
}

function equivalent(left: object, right: object): boolean {
	const before: Record<string, DraftValue> = {};
	const after: Record<string, DraftValue> = {};
	flatten("", left, before);
	flatten("", right, after);
	const keys = new Set([...Object.keys(before), ...Object.keys(after)]);
	for (const key of keys) {
		if (before[key] !== after[key]) return false;
	}
	return true;
}

function summariesFor(
	project: string,
	section: ManifestDraftSection,
	initial: object,
	next: object,
	labels: Record<string, string>,
): ManifestDraftSummary[] {
	const before: Record<string, DraftValue> = {};
	const after: Record<string, DraftValue> = {};
	flatten("", initial, before);
	flatten("", next, after);
	const keys = new Set([...Object.keys(before), ...Object.keys(after)]);
	return [...keys].sort().map((key) => ({
		id: `${project}:${section}:${key}`,
		project,
		section,
		path: key,
		labelKey: labels[key] ?? `manifest.fields.${section}.${key.replaceAll("/", ".")}`,
		before: before[key],
		after: after[key],
		changed: before[key] !== after[key],
	}));
}

export const useManifestDraftStore = createStore<ManifestDraftState>(
	(set) => ({
		drafts: {},
		stageSection: ({ entryId, revision, project, section, initial, next, labels }) => {
			const key = manifestDraftKey(entryId);
			set((state) => {
				const existing = state.drafts[key];
				const changes = { ...existing?.changes };
				const projectPatch = { ...(changes[project] ?? { project }) };
				const summaries = (existing?.summaries ?? []).filter(
					(summary) => !summary.id.startsWith(`${project}:${section}:`),
				);

				if (equivalent(initial, next)) {
					delete projectPatch[section];
				} else {
					projectPatch[section] = next as never;
					summaries.push(...summariesFor(project, section, initial, next, labels));
				}

				const hasProjectChange = ["general", "environment", "container", "deploy"].some(
					(name) => projectPatch[name as ManifestDraftSection] !== undefined,
				);
				if (hasProjectChange) changes[project] = projectPatch;
				else delete changes[project];

				const drafts = { ...state.drafts };
				if (Object.keys(changes).length === 0 && !existing?.workspace) {
					delete drafts[key];
				} else {
					drafts[key] = {
						revision: existing?.revision || revision,
						workspace: existing?.workspace,
						changes,
						summaries,
					};
				}
				return { drafts };
			});
		},
		stageWorkspaceSection: ({ entryId, revision, section, initial, next, labels }) => {
			const key = manifestDraftKey(entryId);
			set((state) => {
				const existing = state.drafts[key];
				const workspace = { ...existing?.workspace };
				const summaries = (existing?.summaries ?? []).filter(
					(summary) => !summary.id.startsWith(`${WORKSPACE_DRAFT_SUBJECT}:${section}:`),
				);

				if (equivalent(initial, next)) {
					delete workspace[section];
				} else {
					workspace[section] = next as WorkspaceManifestPatch["environment"];
					summaries.push(...summariesFor(WORKSPACE_DRAFT_SUBJECT, section, initial, next, labels));
				}

				const hasWorkspaceChange = workspace.environment !== undefined;
				const changes = existing?.changes ?? {};
				const drafts = { ...state.drafts };
				if (!hasWorkspaceChange && Object.keys(changes).length === 0) {
					delete drafts[key];
				} else {
					drafts[key] = {
						revision: existing?.revision || revision,
						workspace: hasWorkspaceChange ? workspace : undefined,
						changes,
						summaries,
					};
				}
				return { drafts };
			});
		},
		commitWorkspaceSection: (entryId, section, revision) => {
			const key = manifestDraftKey(entryId);
			set((state) => {
				const existing = state.drafts[key];
				if (!existing) return state;
				const workspace = { ...existing.workspace };
				delete workspace[section];
				const summaries = existing.summaries.filter(
					(summary) => !summary.id.startsWith(`${WORKSPACE_DRAFT_SUBJECT}:${section}:`),
				);
				const drafts = { ...state.drafts };
				if (Object.keys(existing.changes).length === 0) {
					delete drafts[key];
				} else {
					drafts[key] = {
						revision,
						changes: existing.changes,
						summaries,
					};
				}
				return { drafts };
			});
		},
		clearWorkspace: (entryId) => {
			const key = manifestDraftKey(entryId);
			set((state) => {
				const drafts = { ...state.drafts };
				delete drafts[key];
				return { drafts };
			});
		},
	}),
	"manifestDraftStore",
);

export function displayDraftValue(value: DraftValue): string {
	if (value === undefined || value === null || value === "") return "—";
	if (typeof value === "boolean") return value ? "true" : "false";
	return String(value satisfies ProfileValue);
}
