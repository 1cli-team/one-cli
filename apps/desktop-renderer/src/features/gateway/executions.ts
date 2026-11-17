import useSWR from "swr";
import http from "@/lib/http";
import {
	repositoryRef,
	type CodexModelCatalog,
	type CodexOptions,
	type ProjectSummary,
} from "./api";

export interface ExecutionRepository {
	host: string;
	slug: string;
}

export interface ExecutionWorker {
	id: string;
	name: string;
	online: boolean;
	busy: boolean;
	repositories: ExecutionRepository[] | null;
	models?: CodexModelCatalog;
	linearProjects?: { id: string; repositories: WorkflowRepository[] }[];
}

type WorkflowRepository = Pick<ProjectSummary, "github" | "repository" | "base">;

export function supportsLinear(worker: ExecutionWorker, project: ProjectSummary) {
	if (worker.id === "local") return true;
	const available = worker.linearProjects?.find(({ id }) => id === project.projectId)?.repositories;
	const required = project.repositories?.length ? project.repositories : [project];
	return (
		available?.length === required.length &&
		required.every((repo, index) => {
			const target = available[index];
			const left = repositoryRef(repo);
			const right = repositoryRef(target);
			return (
				left &&
				right &&
				left.host === right.host &&
				left.slug.toLowerCase() === right.slug.toLowerCase() &&
				repo.base === target.base
			);
		})
	);
}

export interface ExecutionInput {
	kind: "one-task" | "codex";
	repository: ExecutionRepository;
	ref?: string;
	commitSha?: string;
	project?: string;
	task?: string;
	prompt?: string;
	codex?: CodexOptions;
	publishRef?: boolean;
}

export interface ExecutionSubmission {
	workerId: string;
	requestId: string;
	input: ExecutionInput;
}

export const executionStates = [
	"queued",
	"running",
	"waiting_for_answer",
	"waiting_to_continue",
	"outcome_unknown",
	"succeeded",
	"failed",
] as const;
export type ExecutionState = (typeof executionStates)[number];

export interface ExecutionQuestion {
	id: string;
	text: string;
	answer?: string;
}

export interface ExecutionProgress {
	id: string;
	summary: string;
	continueRequested?: boolean;
}

export interface Execution extends Omit<ExecutionSubmission, "input"> {
	input:
		| ExecutionInput
		| {
				kind: "linear-workflow";
				linear: { identifier: string; title: string; url: string };
		  };
	id: string;
	source: "desktop" | "cli";
	state: ExecutionState;
	questions?: ExecutionQuestion[];
	progress?: ExecutionProgress;
	reason?: "worker_offline" | "worker_disconnected";
	result?: {
		exitCode?: number;
		commitSha?: string;
		ref?: string;
		response?: string;
		threadId?: string;
		pullRequests?: { number: number; url: string }[];
		error?: string;
	};
}

interface ExecutionLogs {
	data: string | null;
	truncated: boolean;
}

export interface WorkerInvite {
	url: string;
	code: string;
	expiresAt: string;
}

export function useWorkers(connection: string | undefined) {
	return useSWR(
		connection ? ["/gateway/workers", connection] : null,
		([url]) => http.get<ExecutionWorker[]>(url),
		{
			refreshInterval: 4000,
		},
	);
}

export function useExecutions(connection: string) {
	return useSWR(["/gateway/executions", connection], ([url]) => http.get<Execution[]>(url), {
		refreshInterval: 4000,
	});
}

export function useExecution(connection: string, id: string) {
	const path = `/gateway/executions/${encodeURIComponent(id)}`;
	const job = useSWR([path, connection], ([url]) => http.get<Execution>(url), {
		refreshInterval: (current) =>
			current?.state === "succeeded" || current?.state === "failed" ? 0 : 2000,
	});
	const logs = useSWR([`${path}/logs`, connection], ([url]) => http.get<ExecutionLogs>(url), {
		refreshInterval: 2500,
	});
	return { job, logs };
}

export function inviteWorker() {
	return http.post<WorkerInvite>("/gateway/workers/invite");
}

export function answerExecution(id: string, questionId: string, answer: string) {
	return http.post<ExecutionQuestion>(`/gateway/executions/${encodeURIComponent(id)}/answer`, {
		questionId,
		answer,
	});
}

export function continueExecution(id: string, progressId: string) {
	return http.post<ExecutionProgress>(`/gateway/executions/${encodeURIComponent(id)}/continue`, {
		progressId,
	});
}

export function repositoryKey(repo: ExecutionRepository) {
	return `${repo.host}/${repo.slug}`;
}

const ansiSequence = new RegExp(`${String.fromCharCode(27)}\\[[0-?]*[ -/]*[@-~]`, "g");

export function executionLogText(encoded: string) {
	return new TextDecoder()
		.decode(Uint8Array.from(atob(encoded), (value) => value.charCodeAt(0)))
		.replace(ansiSequence, "");
}
