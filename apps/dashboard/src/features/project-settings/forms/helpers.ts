import { overviewKeyFor } from "@/api/workspace";
import type { useToast } from "@/hooks/useToast";
import type { ProfileValue, ProjectProfileBinding } from "@/types/api";

export function projectBindingValue(
	selectedProfile: string | undefined,
	binding: ProjectProfileBinding | undefined,
	environment: string,
): string {
	if (selectedProfile !== undefined) return selectedProfile;
	const directSource = environment ? "workspace-project-environment" : "workspace-project";
	return binding?.source === directSource ? binding.name : "";
}

export function refreshOverview(
	mutate: (key: string) => unknown,
	workspaceEntryId?: string,
	environment?: string,
): void {
	void mutate(overviewKeyFor(workspaceEntryId, environment));
}

export function showSaveError(
	toast: ReturnType<typeof useToast>,
	title: string,
	error: unknown,
): void {
	const failure = error as { code?: string; message?: string };
	toast.error(title, { description: failure.message ?? String(error) });
}

export function configPathValue(
	config: Record<string, ProfileValue>,
	path: string,
): ProfileValue | undefined {
	let value: ProfileValue | undefined = config;
	for (const segment of path.split("/").filter(Boolean)) {
		if (!value || typeof value !== "object" || Array.isArray(value)) return undefined;
		value = (value as Record<string, ProfileValue | undefined>)[segment];
	}
	return value;
}

export function setConfigPathValue(
	config: Record<string, ProfileValue>,
	path: string,
	nextValue: string,
): Record<string, ProfileValue> {
	const result = structuredClone(config);
	const segments = path.split("/").filter(Boolean);
	let current: Record<string, ProfileValue> = result;
	for (const segment of segments.slice(0, -1)) {
		const existing = current[segment];
		if (!existing || typeof existing !== "object" || Array.isArray(existing)) {
			current[segment] = {};
		}
		current = current[segment] as Record<string, ProfileValue>;
	}
	current[segments.at(-1) ?? path] = nextValue;
	return result;
}
