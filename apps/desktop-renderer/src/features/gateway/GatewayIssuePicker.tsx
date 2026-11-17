import { useId, useState } from "react";
import { useTranslation } from "react-i18next";
import useSWR from "swr";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ErrorNotice } from "@/components/ui/page-layout";
import { Choice } from "@/features/linear/Choice";
import {
	errorText,
	fetchJSON,
	gatewayJobsKey,
	scoped,
	type GatewayJobs,
	type IssuePage,
	type Scope,
} from "@/features/linear/api";
import type { ProjectBinding } from "./api";

type Cursors = Record<string, string>;
type IssueQuery = { projectId: string; issuesPath: string; jobsPath: string };

export type IssueSelection = { id: string; projectId: string };

async function loadIssuePages([, queries]: readonly [string, IssueQuery[]]) {
	const results = await Promise.allSettled(
		queries.map(async ({ projectId, issuesPath, jobsPath }) => {
			const [page, jobs] = await Promise.all([
				fetchJSON<IssuePage>(issuesPath),
				fetchJSON<GatewayJobs>(jobsPath).catch(() => undefined),
			]);
			const issues = page.nodes.filter(
				({ id, state }) =>
					state.type !== "completed" && state.type !== "canceled" && !jobs?.issues[id],
			);
			return { projectId, issues, pageInfo: page.pageInfo };
		}),
	);
	const next: Cursors = {};
	const issues = results.flatMap((result) => {
		if (result.status === "rejected") return [];
		const { projectId, issues, pageInfo } = result.value;
		if (pageInfo.hasNextPage && pageInfo.endCursor) next[projectId] = pageInfo.endCursor;
		return issues.map((issue) => ({ ...issue, projectId }));
	});
	return { issues, next, errors: results.filter((result) => result.status === "rejected") };
}

export function GatewayIssuePicker({
	projects,
	value,
	onChange,
	disabled,
}: {
	projects: ProjectBinding[];
	value: string;
	onChange(issue?: IssueSelection): void;
	disabled: boolean;
}) {
	const { t } = useTranslation();
	const searchId = useId();
	const [search, setSearch] = useState("");
	const [query, setQuery] = useState("");
	const [pages, setPages] = useState<(Cursors | undefined)[]>([undefined]);
	const cursors = pages.at(-1);
	const scopes = new Map<string, Scope>(
		projects.map(({ binding }) => [binding.project.id, binding.scope]),
	);
	const queries = Array.from(scopes).flatMap(([projectId, scope]) =>
		cursors && !cursors[projectId]
			? []
			: [
					{
						projectId,
						issuesPath: scoped(scope, "issues", { q: query, after: cursors?.[projectId] ?? "" }),
						jobsPath: gatewayJobsKey(scope),
					},
				],
	);
	const page = useSWR(["gateway-issue-picker", queries] as const, loadIssuePages);
	const issues = page.data?.issues ?? [];
	const nextPage = page.data?.next;
	const hasNextPage = nextPage && Object.keys(nextPage).length > 0;

	return (
		<div className="grid gap-3">
			<form
				className="grid gap-2"
				onSubmit={(event) => {
					event.preventDefault();
					onChange();
					setPages([undefined]);
					const nextQuery = search.trim();
					if (pages.length === 1 && query === nextQuery) void page.mutate();
					setQuery(nextQuery);
				}}
			>
				<Label htmlFor={searchId}>{t("linear.search")}</Label>
				<div className="flex gap-2">
					<Input
						id={searchId}
						type="search"
						value={search}
						disabled={disabled}
						onChange={(event) => setSearch(event.target.value)}
					/>
					<Button type="submit" variant="outline" disabled={disabled}>
						{t("linear.searchAction")}
					</Button>
				</div>
			</form>
			<Choice
				label={t("linear.issue")}
				value={value}
				onChange={(id) => {
					const issue = issues.find((item) => item.id === id);
					onChange(issue && { id: issue.id, projectId: issue.projectId });
				}}
				options={issues.map(({ id, identifier, title, project }) => ({
					id,
					name:
						projects.length === 1
							? `${identifier} · ${title}`
							: `${identifier} · ${title} · ${project.name}`,
				}))}
				placeholder={t(page.isLoading ? "linear.loading" : "gateway.issueActions.selectIssue")}
				disabled={disabled || !issues.length}
			/>
			{page.error && <ErrorNotice>{errorText(page.error)}</ErrorNotice>}
			{page.data?.errors.map(({ reason }, index) => (
				<ErrorNotice key={index}>{errorText(reason)}</ErrorNotice>
			))}
			{page.data && !issues.length && (
				<p className="text-sm text-muted-foreground">{t("gateway.issueActions.noIssues")}</p>
			)}
			{(pages.length > 1 || hasNextPage) && (
				<div className="flex flex-wrap justify-end gap-2">
					<Button
						type="button"
						size="sm"
						variant="outline"
						disabled={disabled || pages.length < 2}
						onClick={() => {
							onChange();
							setPages((current) => current.slice(0, -1));
						}}
					>
						{t("linear.previous")}
					</Button>
					<Button
						type="button"
						size="sm"
						variant="outline"
						disabled={disabled || !hasNextPage}
						onClick={() => {
							if (!nextPage) return;
							onChange();
							setPages((current) => [...current, nextPage]);
						}}
					>
						{t("linear.next")}
					</Button>
				</div>
			)}
		</div>
	);
}
