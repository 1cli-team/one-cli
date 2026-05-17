import { httpClient } from "@/lib/http";
import type { ApiResponse } from "@/types/api";
import type { User } from "@/api/users";

export interface LoginRequest {
	email: string;
	password: string;
}

export interface LoginResponse {
	token: string;
	user: User;
}

export const loginKey = "/auth/login";

export function login(data: LoginRequest): Promise<ApiResponse<LoginResponse>> {
	return httpClient.post(loginKey, data);
}

export const logoutKey = "/auth/logout";

export function logout(): Promise<ApiResponse<null>> {
	return httpClient.post(logoutKey);
}

export const refreshTokenKey = "/auth/refresh";

export function refreshToken(): Promise<ApiResponse<{ token: string }>> {
	return httpClient.post(refreshTokenKey);
}

export const currentUserKey = "/auth/me";

export function getCurrentUser(): Promise<ApiResponse<User>> {
	return httpClient.get(currentUserKey);
}
