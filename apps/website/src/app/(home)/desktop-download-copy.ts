import type { Locale } from "@/i18n";

const en = {
	title: "Download One Desktop",
	description: "Manage workspaces, projects, and shared credentials on your desktop.",
	stable: "Latest stable release",
	system: "Choose your computer’s operating system",
	macos: "macOS",
	windows: "Windows",
	linux: "Linux",
	hints: {
		macos: "Choose your Mac’s chip. Find it in Apple menu → About This Mac.",
		windows: "For Windows PCs with an x64 processor.",
		linux: "Choose the AppImage that matches your computer’s processor.",
	},
	loading: "Getting the latest installers…",
	errors: {
		network: "Could not connect to GitHub. Check your connection and try again.",
		rateLimited: "GitHub is temporarily limiting requests. Try again later or download on GitHub.",
		unavailable: "The latest installers are unavailable. Try again or view the release on GitHub.",
	},
	empty:
		"This release has no installer for this system. Choose another system or view the release on GitHub.",
	retry: "Try again",
	download: "Download",
	downloadLabel: "Download One Desktop for {system} ({architecture})",
	github: "View release on GitHub",
	close: "Close download dialog",
};

export const desktopDownloadCopy = {
	en,
	zh: {
		title: "下载 One Desktop",
		description: "在桌面上管理工作区、项目和共享凭据。",
		stable: "最新正式版",
		system: "选择电脑的操作系统",
		macos: "macOS",
		windows: "Windows",
		linux: "Linux",
		hints: {
			macos: "请选择 Mac 的芯片类型，可在苹果菜单 → 关于本机中查看。",
			windows: "适用于使用 x64 处理器的 Windows 电脑。",
			linux: "请选择与电脑处理器匹配的 AppImage 安装包。",
		},
		loading: "正在获取最新安装包…",
		errors: {
			network: "无法连接 GitHub，请检查网络后重试。",
			rateLimited: "GitHub 暂时限制了请求，请稍后重试，或前往 GitHub 下载。",
			unavailable: "暂时无法获取最新安装包，请重试，或前往 GitHub 查看此版本。",
		},
		empty: "此版本暂无适用于该系统的安装包，请选择其他系统，或前往 GitHub 查看。",
		retry: "重试",
		download: "下载",
		downloadLabel: "下载适用于 {system}（{architecture}）的 One Desktop",
		github: "前往 GitHub 查看版本",
		close: "关闭下载弹窗",
	},
} satisfies Record<Locale, typeof en>;
