export interface DemoResponse {
	message: string;
	timestamp: string;
	count: number;
	status: string;
}

export const demoKey = "/api/demo";

export async function getDemo(): Promise<DemoResponse> {
	await new Promise((resolve) => setTimeout(resolve, 900));

	return {
		message: "SWR cache 已同步完成",
		timestamp: new Date().toLocaleString("zh-CN"),
		count: Math.floor(Math.random() * 100),
		status: "success",
	};
}
