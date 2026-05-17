import { type NextRequest, NextResponse } from "next/server";

export async function GET(request: NextRequest) {
	const { searchParams } = new URL(request.url);
	const showError = searchParams.get("error") === "true";

	// Simulate a delay
	await new Promise((resolve) => setTimeout(resolve, 500));

	if (showError) {
		// Return a business error response
		return NextResponse.json({
			code: "01001", // Business domain 01, error 001
			msg: "模拟的业务错误：参数验证失败",
			data: null,
		});
	}

	// Return a success response with server data structure
	return NextResponse.json({
		code: "00000", // Success code
		msg: "请求成功",
		data: {
			message: "Hello from Client Side API!",
			timestamp: new Date().toISOString(),
			serverInfo: {
				nodeVersion: process.version,
				platform: process.platform,
			},
		},
	});
}
