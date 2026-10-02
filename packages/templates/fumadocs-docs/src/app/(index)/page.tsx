import Link from "next/link";
export default function Page() {
	return (
		<main className="mx-auto flex min-h-screen max-w-2xl flex-col items-center justify-center gap-6 px-6 text-center">
			<h1 className="text-4xl font-semibold">My Docs / 我的文档</h1>
			<p className="text-muted-foreground">Choose your language / 选择语言</p>
			<nav aria-label="Language / 语言" className="flex gap-6">
				<Link className="text-primary underline" href="/en/docs/">
					English
				</Link>
				<Link className="text-primary underline" href="/zh/docs/" lang="zh-CN">
					中文
				</Link>
			</nav>
		</main>
	);
}
