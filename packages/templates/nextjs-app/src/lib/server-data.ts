// 服务端数据获取函数
// 这些函数只在服务器端运行，支持 SSR
import type { HelloResponse } from "@/api/hello";

export async function getHelloData(): Promise<HelloResponse> {
	// 模拟 API 调用延迟
	await new Promise((resolve) => setTimeout(resolve, 100));

	return {
		message: "Hello from Server Side Rendering!",
		timestamp: new Date().toISOString(),
		serverInfo: {
			nodeVersion: process.version,
			platform: process.platform,
		},
	};
}

export async function getUserData(userId?: string) {
	// 模拟用户数据获取
	await new Promise((resolve) => setTimeout(resolve, 200));

	return {
		id: userId || "1",
		name: "张三",
		email: "zhangsan@example.com",
		avatar: "https://github.com/shadcn.png",
		createdAt: new Date().toISOString(),
		preferences: {
			theme: "system",
			language: "zh-CN",
		},
	};
}

// 模拟可能失败的 API 调用
export async function getDataWithError(shouldError: boolean = false) {
	await new Promise((resolve) => setTimeout(resolve, 150));

	if (shouldError) {
		throw new Error("模拟的服务器错误");
	}

	return {
		success: true,
		data: "成功获取的数据",
		timestamp: new Date().toISOString(),
	};
}
