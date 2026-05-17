import http from "@/lib/http";

export interface HelloResponse {
	message: string;
	timestamp: string;
	serverInfo: {
		nodeVersion: string;
		platform: string;
	};
}

export const helloKey = "/api/hello";
export const helloErrorKey = `${helloKey}?error=true`;

export function getHello(): Promise<HelloResponse> {
	return http.get<HelloResponse, HelloResponse>(helloKey);
}

export function getHelloWithError(): Promise<HelloResponse> {
	return http.get<HelloResponse, HelloResponse>(helloErrorKey);
}
