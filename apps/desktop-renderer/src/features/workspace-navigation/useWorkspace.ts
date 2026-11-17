import { useLocation } from "react-router-dom";
import useSWR from "swr";
import { getOverview, overviewKeyFor } from "@/api/workspace";
import { getWorkspaces, workspacesKey } from "@/api/workspaces";
import { environmentFromSearch } from "@/features/environment-context/environment";

export function useWorkspace(entryId?: string) {
	const { search } = useLocation();
	const environment = environmentFromSearch(search);
	const registry = useSWR(workspacesKey, getWorkspaces);
	const workspace = registry.data?.workspaces.find((entry) => entry.entryId === entryId);
	const available = workspace?.status === "ready" || workspace?.status === "identity-conflict";
	const overview = useSWR(
		entryId && available ? overviewKeyFor(entryId, environment) : null,
		() => getOverview(entryId!, environment),
		{ shouldRetryOnError: false },
	);
	return { registry, workspace, overview, environment, available };
}
