import type { MetadataRoute } from "next";
import { siteUrl } from "@/lib/seo";

export const dynamic = "force-static";

export default function sitemap(): MetadataRoute.Sitemap {
	const languages = {
		"zh-Hans": `${siteUrl}/zh/`,
		en: `${siteUrl}/en/`,
		"x-default": `${siteUrl}/zh/`,
	};
	return ["/", "/zh/", "/en/"].map((path) => ({
		url: new URL(path, siteUrl).toString(),
		alternates: { languages },
	}));
}
