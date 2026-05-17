import axios, { type AxiosInstance, type AxiosResponse } from "axios";

// --- 1. 定义 API 响应和错误结构 ---

// API 响应的基础结构
interface BaseResponse<T> {
	code: string;
	msg: string;
	data: T;
}

// 自定义业务错误类
export class ApiError extends Error {
	public readonly code: string;
	public readonly data: unknown;

	constructor(code: string, message: string, data: unknown = null) {
		super(message);
		this.name = "ApiError";
		this.code = code;
		this.data = data;
	}
}

// --- 2. 创建和配置 Axios 实例 ---

const http: AxiosInstance = axios.create({
	// baseURL: process.env.NEXT_PUBLIC_API_URL, // 如果有统一的 API 地址
	timeout: 10000, // 请求超时时间
	headers: {
		"Content-Type": "application/json",
	},
});

// --- 3. 设置响应拦截器 ---

http.interceptors.response.use(
	(response: AxiosResponse<BaseResponse<any>>) => {
		const { code, msg, data } = response.data;
		// 假设 '00000' 是成功的业务代码
		if (code === "00000") {
			// 直接返回业务数据
			return data as any;
		}

		// 如果是业务错误，则抛出 ApiError
		return Promise.reject(new ApiError(code, msg, data as any));
	},
	(error) => {
		// 处理网络层面的错误 (e.g., 404, 500)
		// 你可以在这里添加更复杂的逻辑，例如根据状态码跳转页面
		if (error.response) {
			console.error("API Error Response:", error.response);
		} else if (error.request) {
			console.error("API Error Request:", error.request);
		} else {
			console.error("API Error Message:", error.message);
		}
		// 抛出原始的 axios 错误，或者一个更友好的错误对象
		return Promise.reject(error);
	},
);

export default http;
