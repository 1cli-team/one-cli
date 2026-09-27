import { overviewKeyFor } from "@/api/workspace";
import type { useToast } from "@/hooks/useToast";
import type { JsonValue } from "@/types/api";

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
	config: Record<string, JsonValue>,
	path: string,
): JsonValue | undefined {
	let value: JsonValue | undefined = config;
	for (const segment of path.split("/").filter(Boolean)) {
		if (!value || typeof value !== "object" || Array.isArray(value)) return undefined;
		value = (value as Record<string, JsonValue | undefined>)[segment];
	}
	return value;
}

export function setConfigPathValue(
	config: Record<string, JsonValue>,
	path: string,
	nextValue: string,
): Record<string, JsonValue> {
	const result = structuredClone(config);
	const segments = path.split("/").filter(Boolean);
	let current: Record<string, JsonValue> = result;
	for (const segment of segments.slice(0, -1)) {
		const existing = current[segment];
		if (!existing || typeof existing !== "object" || Array.isArray(existing)) {
			current[segment] = {};
		}
		current = current[segment] as Record<string, JsonValue>;
	}
	current[segments.at(-1) ?? path] = nextValue;
	return result;
}
