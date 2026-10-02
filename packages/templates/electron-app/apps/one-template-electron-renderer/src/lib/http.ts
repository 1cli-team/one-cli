import axios, { type AxiosRequestConfig } from "axios";

// axios 实例：统一 baseURL / timeout / interceptors 的地方
// 业务代码从这里 import http，不要直接用 axios
export const http = axios.create({
  timeout: 10_000,
});

// 给 swr 用的通用 fetcher
// useSWR("/api/xxx", fetcher) 就能直接使用
export const fetcher = async <T = unknown>(
  url: string,
  config?: AxiosRequestConfig,
): Promise<T> => {
  const res = await http.get<T>(url, config);
  return res.data;
};
