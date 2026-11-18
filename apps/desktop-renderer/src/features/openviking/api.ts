import http from "@/lib/http";

export interface KnowledgeBinding {
	uri: string;
}

export interface KnowledgeStatus {
	configured: boolean;
	needsRebind: boolean;
	binding: KnowledgeBinding | null;
}

export interface KnowledgeMatch {
	uri: string;
	abstract: string;
	score: number;
}

export interface KnowledgeContent {
	uri: string;
	text: string;
	isDirectory: boolean;
}

export const knowledgeKey = (entryId: string) =>
	"/workspaces/" + encodeURIComponent(entryId) + "/openviking";

export const getKnowledge = (key: string) => http.get<KnowledgeStatus>(key);

export const bindKnowledge = (key: string, input: KnowledgeBinding) =>
	http.put<KnowledgeStatus>(key, input);

export const unbindKnowledge = (key: string) => http.delete<KnowledgeStatus>(key);

export const findKnowledge = (key: string, query: string) =>
	http.get<KnowledgeMatch[]>(key + "/find", { params: { query } });

export const readKnowledge = (key: string, uri: string) =>
	http.get<KnowledgeContent>(key + "/content", { params: { uri } });

export interface KnowledgeImport {
	taskId: string;
	uri?: string;
	status: "pending" | "running" | "cancelling" | "completed" | "failed" | "cancelled";
	error?: string;
}

export const MAX_KNOWLEDGE_IMPORT_SIZE = 20 * 1024 * 1024;

export const importKnowledge = (key: string, file: File) =>
	http.post<KnowledgeImport>(key + "/imports", file, {
		params: { name: file.name },
		headers: { "Content-Type": "application/octet-stream" },
		timeout: 190_000,
	});

export const getImportStatus = (key: string, taskId: string) =>
	http.get<KnowledgeImport>(key + "/imports/" + encodeURIComponent(taskId));

export interface KnowledgeDirectories {
	current: string;
	parent?: string;
	entries: { name: string; uri: string }[];
}

export const browseKnowledge = (key: string, input: KnowledgeBinding) =>
	http.post<KnowledgeDirectories>(key + "/directories", input, { timeout: 18_000 });

export interface OpenVikingConnection {
	url: string;
	hasApiKey: boolean;
	account?: string;
	user?: string;
}

type ConnectionInput = { url: string; apiKey?: string };

export const connectionKey = "/openviking/connection";
export const getConnection = (key: string) => http.get<OpenVikingConnection | null>(key);
export const saveConnection = (input: ConnectionInput) =>
	http.put<OpenVikingConnection>(connectionKey, input, { timeout: 16_000 });
export const testConnection = (input: ConnectionInput) =>
	http.post<OpenVikingConnection>(connectionKey + "/test", input, { timeout: 16_000 });

export type SkillScope = "user" | "shared";

export interface SkillRef {
	name: string;
	scope: SkillScope;
}

export interface KnowledgeSkill extends SkillRef {
	description: string;
	uri: string;
	content?: string;
	attachments?: boolean;
}

export interface WorkspaceAgent {
	automatic: boolean;
	skills: SkillRef[] | null;
}

export interface SessionLink {
	id: string;
	workspaceId: string;
	taskId?: string;
	status?: string;
}

export interface KnowledgeSession {
	id: string;
	title?: string;
	threadId?: string;
	archives: number;
	archive?: string;
	messages?: { role: string; text: string }[];
	taskId?: string;
	status?: string;
	memoryCount?: number;
}

export const getWorkspaceAgent = (endpoint: string) =>
	http.get<WorkspaceAgent>(endpoint + "/agent");

export const setWorkspaceAgent = (
	endpoint: string,
	input: { automatic: boolean } | { skill: SkillRef; enabled: boolean },
) => http.put<WorkspaceAgent>(endpoint + "/agent", input);

export const getSessions = (endpoint: string) =>
	http.get<KnowledgeSession[]>(endpoint + "/sessions");

export const getSession = (endpoint: string, id: string, archive?: string) =>
	http.get<KnowledgeSession>(endpoint + "/sessions/" + encodeURIComponent(id), {
		params: { archive },
		timeout: 28_000,
	});

export const listMemories = (endpoint: string, query: string) =>
	http.get<KnowledgeMatch[]>(endpoint + "/memories", { params: { query } });

export const readMemory = (endpoint: string, uri: string) =>
	http.get<KnowledgeContent>(endpoint + "/memory", { params: { uri } });

export const writeMemory = (endpoint: string, text: string) =>
	http.post<KnowledgeSession>(endpoint + "/memories", { text }, { timeout: 55_000 });

export const getSkills = (endpoint: string, query: string) =>
	http.get<KnowledgeSkill[]>(endpoint + "/skills", { params: { query } });

export const getSkill = (endpoint: string, skill: SkillRef) =>
	http.get<KnowledgeSkill>(endpoint + "/skills/" + encodeURIComponent(skill.name), {
		params: { scope: skill.scope },
	});

export const saveSkill = (
	endpoint: string,
	input: { scope: SkillScope; source: string; name?: string; replace?: boolean },
) => http.post<{ taskId: string }>(endpoint + "/skills", input, { timeout: 55_000 });

export const deleteSkill = (endpoint: string, skill: SkillRef) =>
	http.delete(endpoint + "/skills/" + encodeURIComponent(skill.name), {
		params: { scope: skill.scope },
	});
