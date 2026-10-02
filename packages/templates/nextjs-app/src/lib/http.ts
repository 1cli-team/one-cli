import axios from "axios";

export const http = axios.create({ baseURL: process.env.NEXT_PUBLIC_API_URL, timeout: 10000 });
export async function fetcher<T>(url: string): Promise<T> {
	return (await http.get<T>(url)).data;
}
