import type { To } from "react-router-dom";

export const DASHBOARD_ENVIRONMENTS = ["dev", "preview", "prod"] as const;

export type DashboardEnvironment = (typeof DASHBOARD_ENVIRONMENTS)[number];

export function isDashboardEnvironment(value: string | null): value is DashboardEnvironment {
	return DASHBOARD_ENVIRONMENTS.some((environment) => environment === value);
}

export function environmentFromSearch(search: string): DashboardEnvironment {
	const requested = new URLSearchParams(search).get("env");
	return isDashboardEnvironment(requested) ? requested : "dev";
}

/** Keep an explicitly selected environment while navigating between Dashboard pages. */
export function preserveEnvironment(to: To, currentSearch: string): To {
	const requested = new URLSearchParams(currentSearch).get("env");
	if (!isDashboardEnvironment(requested)) return to;

	if (typeof to === "string") {
		const target = new URL(to, "http://one.local");
		target.searchParams.set("env", requested);
		return `${target.pathname}${target.search}${target.hash}`;
	}

	const targetSearch = new URLSearchParams(to.search);
	targetSearch.set("env", requested);
	return { ...to, search: `?${targetSearch.toString()}` };
}
