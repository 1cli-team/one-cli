import { useLocation, useParams } from "react-router-dom";
import useSWR from "swr";
import { getOverview, overviewKeyFor } from "@/api/workspace";
import { getWorkspaces, workspacesKey } from "@/api/workspaces";
import { environmentFromSearch } from "@/features/environment-context/environment";
import { workspaceContext } from "@/router/paths";
import type { WorkspaceRegistryStatus } from "@/types/api";

export function useWorkspace(entryId?: string) {
	const { search } = useLocation();
	const { worktreeId } = useParams();
	const environment = environmentFromSearch(search);
	const registry = useSWR(workspacesKey, getWorkspaces);
	const logicalId = entryId?.split("~")[0];
	const registered = registry.data?.workspaces.find(
		(entry) =>
			entry.entryId === logicalId || entry.aliases?.some((alias) => alias.entryId === logicalId),
	);
	const alias = registered?.aliases?.find((item) => item.entryId === logicalId);
	const selectedId =
		worktreeId || entryId?.split("~")[1] || alias?.worktreeId || registered?.worktreeId;
	const selected = registered?.worktrees?.find((tree) => tree.id === selectedId);
	const selectedStatus: WorkspaceRegistryStatus | undefined =
		selectedId && registered?.worktrees?.length && !selected ? "missing" : selected?.status;
	const contextEntryId = workspaceContext(registered?.entryId || entryId || "", selectedId);
	const workspace = registered
		? {
				...registered,
				entryId: contextEntryId,
				root: selected?.root ?? registered.root,
				worktreeId: selectedId,
				status:
					registered.status === "identity-conflict"
						? registered.status
						: (selectedStatus ?? registered.status),
				projectCount: selected?.projectCount ?? registered.projectCount,
			}
		: undefined;
	const available = workspace?.status === "ready" || workspace?.status === "identity-conflict";
	const overview = useSWR(
		entryId && available ? overviewKeyFor(contextEntryId, environment) : null,
		() => getOverview(contextEntryId, environment),
		{ shouldRetryOnError: false },
	);
	return { registry, workspace, registered, contextEntryId, overview, environment, available };
}
