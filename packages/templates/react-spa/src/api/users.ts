import { httpClient } from "@/lib/http";
import type { ApiResponse, PaginationParams, PaginationResponse } from "@/types/api";

export interface User {
	id: number;
	name: string;
	email: string;
	avatar?: string;
	createdAt: string;
}

export interface CreateUserRequest {
	name: string;
	email: string;
}

export const usersKey = "/users";

export function getUsers(params: PaginationParams): Promise<ApiResponse<PaginationResponse<User>>> {
	return httpClient.get(usersKey, { params });
}

export function getUserKey(id: number) {
	return `${usersKey}/${id}`;
}

export function getUserById(id: number): Promise<ApiResponse<User>> {
	return httpClient.get(getUserKey(id));
}

export function createUser(data: CreateUserRequest): Promise<ApiResponse<User>> {
	return httpClient.post(usersKey, data);
}

export function updateUser(
	id: number,
	data: Partial<CreateUserRequest>,
): Promise<ApiResponse<User>> {
	return httpClient.put(getUserKey(id), data);
}

export function deleteUser(id: number): Promise<ApiResponse<null>> {
	return httpClient.delete(getUserKey(id));
}
