import type { To } from "react-router-dom";

export const DASHBOARD_ENVIRONMENTS = ["dev", "staging", "prod"] as const;

export type DashboardEnvironment = string;

export function isDashboardEnvironment(value: string | null): value is DashboardEnvironment {
	return value !== null && /^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$/.test(value);
}

export function environmentFromSearch(search: string): DashboardEnvironment {
	const requested = new URLSearchParams(search).get("env");
	return isDashboardEnvironment(requested) ? requested : "dev";
}

/** Keep environment and project context when navigating between workspace pages. */
export function preserveEnvironment(to: To, currentSearch: string): To {
	const current = new URLSearchParams(currentSearch);
	const requested = current.get("env");
	const group = current.get("group");
	const target = typeof to === "string" ? new URL(to, "http://one.local") : to;
	const carriesGroup = Boolean(group && target.pathname?.startsWith("/workspaces/"));
	if (!isDashboardEnvironment(requested) && !carriesGroup) return to;
	const parameters = new URLSearchParams(target.search);
	if (isDashboardEnvironment(requested)) parameters.set("env", requested);
	if (group && carriesGroup && !parameters.has("group")) {
		parameters.set("group", group);
	}
	const search = parameters.size ? `?${parameters}` : "";
	return typeof to === "string"
		? `${target.pathname}${search}${target.hash ?? ""}`
		: { ...to, search };
}
