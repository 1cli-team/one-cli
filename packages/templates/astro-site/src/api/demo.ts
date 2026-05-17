import { api, type ApiResponse } from "../utils/api";

export interface DemoPost {
  userId: number;
  id?: number;
  title: string;
  body: string;
}

export interface CreateDemoPostRequest {
  title: string;
  body: string;
  userId: number;
}

export const demoPostKey = "https://jsonplaceholder.typicode.com/posts/1";
export const createDemoPostKey = "https://jsonplaceholder.typicode.com/posts";
export const brokenDemoRequestKey = "https://nonexistent-api-endpoint-12345.com/test";

export function getDemoPost(): Promise<ApiResponse<DemoPost>> {
  return api.get<DemoPost>(demoPostKey);
}

export function createDemoPost(data: CreateDemoPostRequest): Promise<ApiResponse<DemoPost>> {
  return api.post<DemoPost>(createDemoPostKey, data);
}

export function getBrokenDemoRequest(): Promise<ApiResponse<unknown>> {
  return api.get(brokenDemoRequestKey);
}
