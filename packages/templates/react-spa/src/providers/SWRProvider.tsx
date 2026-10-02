import type { PropsWithChildren } from "react";
import { SWRConfig } from "swr";
import { fetcher } from "@/lib/http";

export function SWRProvider({ children }: PropsWithChildren) {
	return <SWRConfig value={{ fetcher }}>{children}</SWRConfig>;
}
