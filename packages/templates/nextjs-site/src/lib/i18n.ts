export const locales = ["en", "zh"] as const;
export type Locale = (typeof locales)[number];
export const htmlLang = { en: "en-US", zh: "zh-CN" };
export function isLocale(value: string): value is Locale {
	return locales.includes(value as Locale);
}
export const messages = {
	en: {
		name: "My Website",
		title: "Build something worth sharing",
		description: "A React website ready for your product, company, or next idea.",
		home: "Home",
		about: "About",
		start: "Explore the site",
		theme: "Change theme",
		light: "Light",
		dark: "Dark",
		system: "System",
		aboutTitle: "A simple place to tell your story",
		aboutDescription: "Add your company story, product details, or latest work here.",
		featureTitle: "Ready for your content",
		featureDescription:
			"Publish pages with a shared layout, accessible components, and consistent design tokens.",
		notice: "Welcome",
		noticeDescription: "Your website is ready to customize.",
		notify: "Try a notification",
		back: "Back to home",
		notFound: "Page not found",
	},
	zh: {
		name: "我的网站",
		title: "让值得分享的想法成为网站",
		description: "一个为产品、公司和新想法准备好的 React 网站。",
		home: "首页",
		about: "关于",
		start: "探索网站",
		theme: "切换主题",
		light: "浅色",
		dark: "深色",
		system: "跟随系统",
		aboutTitle: "在这里讲述你的故事",
		aboutDescription: "添加公司介绍、产品信息或最新作品。",
		featureTitle: "为你的内容做好准备",
		featureDescription: "使用共享布局、无障碍组件和统一设计令牌发布页面。",
		notice: "欢迎",
		noticeDescription: "你的网站已准备好，可以开始定制。",
		notify: "试试通知",
		back: "返回首页",
		notFound: "页面不存在",
	},
};
