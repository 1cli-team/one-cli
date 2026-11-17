import { useMemo } from "react";
import useSWR from "swr";
import {
	getWorkspaceGroups,
	normalizeWorkspaceGroups,
	workspaceGroupsKey,
} from "@/api/workspace-groups";

export function useWorkspaceGroups() {
	const result = useSWR(workspaceGroupsKey, getWorkspaceGroups, { revalidateOnFocus: true });
	// A mounted SWR cache can still hold the previous API shape after Vite HMR.
	const data = useMemo(
		() => (result.data ? normalizeWorkspaceGroups(result.data) : undefined),
		[result.data],
	);
	return { ...result, data };
}
