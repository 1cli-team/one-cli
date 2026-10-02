import "./globals.css";
export default function NotFound() {
	return (
		<html lang="en">
			<body className="flex min-h-screen flex-col items-center justify-center gap-6">
				<h1 className="text-3xl font-semibold">Page not found / 页面不存在</h1>
				<a href="/" className="text-primary underline">
					Back to home / 返回首页
				</a>
			</body>
		</html>
	);
}
