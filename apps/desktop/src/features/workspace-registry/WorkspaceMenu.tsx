import { Check } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useMatch } from "react-router-dom";
import useSWR from "swr";
import { getWorkspaces, workspacesKey } from "@/api/workspaces";
import {
	DropdownMenuGroup,
	DropdownMenuItem,
	DropdownMenuLabel,
} from "@/components/ui/dropdown-menu";
import { EnvironmentLink } from "@/features/environment-context/EnvironmentLink";
import { cn } from "@/lib/utils";
import type { WorkspaceRegistryStatus } from "@/types/api";

const STATUS_DOT_CLASS: Record<WorkspaceRegistryStatus, string> = {
	ready: "bg-success-foreground",
	missing: "bg-muted-foreground",
	invalid: "bg-error-foreground",
	"identity-missing": "bg-warning-foreground",
	"identity-conflict": "bg-warning-foreground",
};

export function WorkspaceMenu() {
	const { t } = useTranslation();
	const activeEntryId = useMatch("/workspace/:entryId")?.params.entryId;
	const registry = useSWR(workspacesKey, getWorkspaces);
	const workspaces = registry.data?.workspaces ?? [];
	return (
		<>
			<DropdownMenuLabel className="flex items-center justify-between text-xs font-normal text-muted-foreground">
				<span>{t("workspaces.rail.title")}</span>
				<span>{registry.data ? workspaces.length : ""}</span>
			</DropdownMenuLabel>
			<DropdownMenuGroup
				aria-label={t("workspaces.rail.title")}
				className="max-h-64 overflow-y-auto"
			>
				{registry.isLoading && (
					<p role="status" className="px-2 py-3 text-xs text-muted-foreground">
						{t("workspaces.loading")}
					</p>
				)}
				{registry.error && (
					<>
						<p role="alert" className="px-2 py-2 text-xs text-error-foreground">
							{t("workspaces.rail.loadFailed")}
						</p>
						<DropdownMenuItem
							onSelect={(event) => {
								event.preventDefault();
								void registry.mutate().catch(() => undefined);
							}}
						>
							{t("workspaces.retry")}
						</DropdownMenuItem>
					</>
				)}
				{!registry.isLoading && !registry.error && workspaces.length === 0 && (
					<p className="px-2 py-3 text-xs leading-relaxed text-muted-foreground">
						{t("workspaces.rail.empty")}
					</p>
				)}
				{workspaces.map((workspace) => {
					const active = workspace.entryId === activeEntryId;
					return (
						<DropdownMenuItem key={workspace.entryId} asChild>
							<EnvironmentLink
								to={`/workspace/${encodeURIComponent(workspace.entryId)}`}
								aria-current={active ? "page" : undefined}
								title={`${workspace.name}\n${workspace.root}`}
								className="min-h-9 aria-[current=page]:bg-accent aria-[current=page]:text-primary-text"
							>
								<span
									className={cn("size-2 shrink-0 rounded-full", STATUS_DOT_CLASS[workspace.status])}
									role="img"
									aria-label={t(`workspaces.status.${workspace.status}`)}
								/>
								<span className="min-w-0 flex-1 truncate">{workspace.name}</span>
								<span
									className="shrink-0 font-mono text-xs text-muted-foreground"
									title={t("workspaces.home.projectCount", { count: workspace.projectCount })}
								>
									{workspace.projectCount}
								</span>
								{active && <Check className="size-3.5 text-primary-text" aria-hidden="true" />}
							</EnvironmentLink>
						</DropdownMenuItem>
					);
				})}
			</DropdownMenuGroup>
		</>
	);
}
