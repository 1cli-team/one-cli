import axios from "axios";

export const http = axios.create({ baseURL: import.meta.env.VITE_API_URL, timeout: 10000 });
export async function fetcher<T>(url: string): Promise<T> {
	return (await http.get<T>(url)).data;
}
