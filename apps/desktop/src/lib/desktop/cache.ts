import { useMemo } from "react";
import { useSWRConfig } from "swr";
import { useDesktop } from "./runtime";

// Keep existing business keys and predicates while scoping their cache to the
// current backend generation. Bound mutate returned by useSWR already does this.
export function useSessionSWRConfig() {
	const config = useSWRConfig();
	const generation = useDesktop((s) => s.generation);
	const mutate = useMemo(
		() =>
			new Proxy(config.mutate, {
				apply(target, thisArg, [key, ...args]) {
					const scoped =
						typeof key === "function"
							? (candidate: unknown) =>
									Array.isArray(candidate) && candidate[0] === generation && key(candidate[1])
							: [generation, key];
					return Reflect.apply(target, thisArg, [scoped, ...args]);
				},
			}),
		[config.mutate, generation],
	);
	return { ...config, mutate };
}
