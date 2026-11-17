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

export function GatewayIssuePicker({
	scope,
	value,
	onChange,
	disabled,
}: {
	scope: Scope;
	value: string;
	onChange(id: string): void;
	disabled: boolean;
}) {
	const { t } = useTranslation();
	const searchId = useId();
	const [search, setSearch] = useState("");
	const [query, setQuery] = useState("");
	const [cursors, setCursors] = useState([""]);
	const page = useSWR<IssuePage>(
		scoped(scope, "issues", { q: query, after: cursors.at(-1) ?? "" }),
		fetchJSON,
	);
	const jobs = useSWR<GatewayJobs>(gatewayJobsKey(scope), fetchJSON);
	const issues =
		page.data?.nodes.filter(
			({ id, state }) =>
				state.type !== "completed" && state.type !== "canceled" && !jobs.data?.issues[id],
		) ?? [];
	const nextPage = page.data?.pageInfo;

	return (
		<div className="grid gap-3">
			<form
				className="grid gap-2"
				onSubmit={(event) => {
					event.preventDefault();
					onChange("");
					setCursors([""]);
					setQuery(search.trim());
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
				onChange={onChange}
				options={issues.map(({ id, identifier, title }) => ({
					id,
					name: `${identifier} · ${title}`,
				}))}
				placeholder={t(page.isLoading ? "linear.loading" : "gateway.issueActions.selectIssue")}
				disabled={disabled || !issues.length}
			/>
			{page.error && <ErrorNotice>{errorText(page.error)}</ErrorNotice>}
			{page.data && !issues.length && (
				<p className="text-sm text-muted-foreground">{t("gateway.issueActions.noIssues")}</p>
			)}
			{(cursors.length > 1 || nextPage?.hasNextPage) && (
				<div className="flex flex-wrap justify-end gap-2">
					<Button
						type="button"
						size="sm"
						variant="outline"
						disabled={disabled || cursors.length < 2}
						onClick={() => {
							onChange("");
							setCursors((current) => current.slice(0, -1));
						}}
					>
						{t("linear.previous")}
					</Button>
					<Button
						type="button"
						size="sm"
						variant="outline"
						disabled={disabled || !nextPage?.hasNextPage || !nextPage.endCursor}
						onClick={() => {
							if (!nextPage?.endCursor) return;
							onChange("");
							setCursors((current) => [...current, nextPage.endCursor]);
						}}
					>
						{t("linear.next")}
					</Button>
				</div>
			)}
		</div>
	);
}
