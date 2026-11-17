// http.ts wires axios to the Go server in services/desktop-api.
//
// Differences from the source template:
//   - No `{ success, code, message, data }` envelope unwrap. The Go server
//     returns the payload directly (or an `one-cli/error/v1` envelope on
//     failures).
//   - Errors reject with HttpError carrying the full envelope so React UI
//     can surface remediation steps.

import axios, { type AxiosInstance, type AxiosRequestConfig, AxiosError } from "axios";
import i18n from "@/lib/i18n";
import type { ErrorEnvelope, HttpError } from "@/types/api";

class HttpClient {
	private instance: AxiosInstance;

	constructor() {
		this.instance = axios.create({
			withCredentials: false,
			timeout: 10000,
			headers: {
				"Content-Type": "application/json",
			},
		});
	}

	private toHttpError(err: unknown): HttpError {
		if (err instanceof AxiosError && err.response) {
			const status = err.response.status;
			const data = err.response.data as Partial<ErrorEnvelope> | undefined;
			if (data && "error" in data && data.error) {
				return {
					status,
					code: data.error.code,
					message: data.error.message,
					context: data.error.context ?? {},
					remediation: data.error.remediation ?? [],
				};
			}
			return {
				status,
				code: "HTTP_" + status,
				message: err.message,
				context: {},
				remediation: [],
			};
		}
		if (err instanceof AxiosError && err.request) {
			return {
				status: 0,
				code: "NETWORK_ERROR",
				message: err.message || i18n.t("http.networkError"),
				context: {},
				remediation: [],
			};
		}
		return {
			status: 0,
			code: "INTERNAL_ERROR",
			message: err instanceof Error ? err.message : String(err),
			context: {},
			remediation: [],
		};
	}

	private connection: {
		baseURL: string;
		token: string;
		generation: string;
		abort: AbortController;
	} | null = null;
	configure(port: number, token: string, generation: string) {
		if (!Number.isInteger(port) || port < 1 || port > 65535 || !token || !generation)
			throw new Error(i18n.t("desktop.connectionInvalid"));
		if (this.connection?.generation === generation) return;
		this.disconnect();
		this.connection = {
			baseURL: `http://127.0.0.1:${port}/api`,
			token,
			generation,
			abort: new AbortController(),
		};
	}
	disconnect() {
		this.connection?.abort.abort();
		this.connection = null;
	}
	private options(url: string, config?: AxiosRequestConfig): AxiosRequestConfig {
		const c = this.connection;
		if (!c) throw new Error(i18n.t("desktop.notConnected"));
		if (
			!url.startsWith("/") ||
			url.startsWith("//") ||
			url.includes("\\") ||
			new URL(c.baseURL + url).origin !== new URL(c.baseURL).origin ||
			!new URL(c.baseURL + url).pathname.startsWith("/api/")
		)
			throw new Error(i18n.t("desktop.connectionInvalid"));
		return {
			...config,
			baseURL: c.baseURL,
			url,
			signal: c.abort.signal,
			withCredentials: false,
			headers: {
				"Content-Type": "application/json",
				Authorization: `Bearer ${c.token}`,
				"X-One-Backend-Generation": c.generation,
				"X-One-Request-Id": crypto.randomUUID(),
			},
		};
	}
	private async request<T>(
		method: string,
		url: string,
		data?: unknown,
		config?: AxiosRequestConfig,
	): Promise<T> {
		const generation = this.connection?.generation;
		try {
			const response = await this.instance.request<T>({
				...this.options(url, config),
				method,
				data,
			});
			if (generation !== this.connection?.generation)
				throw new Error(i18n.t("desktop.sessionChanged"));
			return response.data;
		} catch (err) {
			throw this.toHttpError(err);
		}
	}
	get<T = unknown>(url: string, config?: AxiosRequestConfig) {
		return this.request<T>("GET", url, undefined, config);
	}
	post<T = unknown>(url: string, body?: unknown, config?: AxiosRequestConfig) {
		return this.request<T>("POST", url, body, config);
	}
	put<T = unknown>(url: string, body?: unknown, config?: AxiosRequestConfig) {
		return this.request<T>("PUT", url, body, config);
	}
	delete<T = unknown>(url: string, config?: AxiosRequestConfig) {
		return this.request<T>("DELETE", url, undefined, config);
	}
}
export const http = new HttpClient();
export default http;
