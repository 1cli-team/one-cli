// http.ts wires axios to the Go server in internal/serve.
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

	constructor(baseURL = "/api") {
		this.instance = axios.create({
			baseURL,
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

	async get<T = unknown>(url: string, config?: AxiosRequestConfig): Promise<T> {
		try {
			const res = await this.instance.get<T>(url, config);
			return res.data;
		} catch (err) {
			throw this.toHttpError(err);
		}
	}

	async post<T = unknown>(url: string, body?: unknown, config?: AxiosRequestConfig): Promise<T> {
		try {
			const res = await this.instance.post<T>(url, body, config);
			return res.data;
		} catch (err) {
			throw this.toHttpError(err);
		}
	}

	async put<T = unknown>(url: string, body?: unknown, config?: AxiosRequestConfig): Promise<T> {
		try {
			const res = await this.instance.put<T>(url, body, config);
			return res.data;
		} catch (err) {
			throw this.toHttpError(err);
		}
	}

	async delete<T = unknown>(url: string, config?: AxiosRequestConfig): Promise<T> {
		try {
			const res = await this.instance.delete<T>(url, config);
			return res.data;
		} catch (err) {
			throw this.toHttpError(err);
		}
	}
}

export const http = new HttpClient();
export default http;
