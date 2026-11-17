import http from "@/lib/http";

export interface CodeHost {
	provider: "github" | "gitea";
	host: string;
	url: string;
	account?: string;
}

export const codeHostsKey = "/code-hosts";
export const getCodeHosts = (path: string) => http.get<CodeHost[]>(path);
export const saveCodeHost = (url: string, token: string) =>
	http.put<CodeHost>(codeHostsKey, { url, token });
export const testCodeHost = (host: string) =>
	http.post<CodeHost>(`${codeHostsKey}/${encodeURIComponent(host)}/test`);
