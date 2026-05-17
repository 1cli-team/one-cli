export interface ApiResponse<T = unknown> {
  data: T;
  status: number;
  statusText: string;
}

export interface ApiError {
  message: string;
  status: number;
  statusText: string;
}

export class ApiException extends Error {
  public status: number;
  public statusText: string;

  constructor(message: string, status: number, statusText: string) {
    super(message);
    this.name = "ApiException";
    this.status = status;
    this.statusText = statusText;
  }
}

const DEFAULT_CONFIG = {
  headers: {
    "Content-Type": "application/json",
  },
};

async function handleResponse<T>(response: Response): Promise<ApiResponse<T>> {
  const isJson = response.headers.get("content-type")?.includes("application/json");
  const data = isJson ? await response.json() : await response.text();

  if (!response.ok) {
    throw new ApiException(
      data?.message || `HTTP Error: ${response.status}`,
      response.status,
      response.statusText,
    );
  }

  return {
    data,
    status: response.status,
    statusText: response.statusText,
  };
}

export async function get<T = unknown>(
  url: string,
  config: RequestInit = {},
): Promise<ApiResponse<T>> {
  const response = await fetch(url, {
    ...DEFAULT_CONFIG,
    ...config,
    method: "GET",
    headers: {
      ...DEFAULT_CONFIG.headers,
      ...config.headers,
    },
  });

  return handleResponse<T>(response);
}

export async function post<T = unknown>(
  url: string,
  data?: unknown,
  config: RequestInit = {},
): Promise<ApiResponse<T>> {
  const response = await fetch(url, {
    ...DEFAULT_CONFIG,
    ...config,
    method: "POST",
    headers: {
      ...DEFAULT_CONFIG.headers,
      ...config.headers,
    },
    body: data ? JSON.stringify(data) : undefined,
  });

  return handleResponse<T>(response);
}

export async function put<T = unknown>(
  url: string,
  data?: unknown,
  config: RequestInit = {},
): Promise<ApiResponse<T>> {
  const response = await fetch(url, {
    ...DEFAULT_CONFIG,
    ...config,
    method: "PUT",
    headers: {
      ...DEFAULT_CONFIG.headers,
      ...config.headers,
    },
    body: data ? JSON.stringify(data) : undefined,
  });

  return handleResponse<T>(response);
}

export async function del<T = unknown>(
  url: string,
  config: RequestInit = {},
): Promise<ApiResponse<T>> {
  const response = await fetch(url, {
    ...DEFAULT_CONFIG,
    ...config,
    method: "DELETE",
    headers: {
      ...DEFAULT_CONFIG.headers,
      ...config.headers,
    },
  });

  return handleResponse<T>(response);
}

export const api = {
  get,
  post,
  put,
  delete: del,
};
