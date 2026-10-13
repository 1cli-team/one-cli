import Link from "next/link";
import { SiteDocument } from "@/components/site-document";

export default function NotFound() {
	return (
		<SiteDocument locale="en">
			<main className="flex min-h-screen flex-col items-center justify-center gap-6 bg-[#0a0a0a] px-5 text-center text-stone-100">
				<h1 className="text-5xl font-bold">404</h1>
				<p>
					Page not found. <span lang="zh-Hans">页面不存在。</span>
				</p>
				<nav className="flex gap-6 text-orange-400" aria-label="Home / 首页">
					<Link href="/zh/" lang="zh-Hans">
						返回首页
					</Link>
					<Link href="/en/">Back to home</Link>
				</nav>
			</main>
		</SiteDocument>
	);
}
