import type React from "react";
import { SWRConfig, type Middleware } from "swr";
import { useDesktop } from "@/lib/desktop/runtime";

const sessionCache: Middleware = (next) => (key, fetcher, config) => {
	const generation = useDesktop((s) => s.generation);
	const connected = useDesktop((s) => s.connected);
	const resolved = typeof key === "function" ? key() : key;
	return next(
		connected && resolved ? [generation, resolved] : null,
		fetcher ? ([, original]: [string, unknown]) => fetcher(original) : null,
		config,
	);
};

export const SWRProvider: React.FC<React.PropsWithChildren> = ({ children }) => {
	return (
		<SWRConfig
			value={{
				use: [sessionCache],
				refreshInterval: 0,
				revalidateOnFocus: false,
				revalidateOnReconnect: true,
				shouldRetryOnError: true,
				errorRetryCount: 3,
				errorRetryInterval: 5000,
				dedupingInterval: 2000,
			}}
		>
			{children}
		</SWRConfig>
	);
};
