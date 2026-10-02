import type { MetadataRoute } from "next";
import { locales } from "@/lib/i18n";
export const dynamic = "force-static";
export default function sitemap(): MetadataRoute.Sitemap {
	const base = (process.env.NEXT_PUBLIC_SITE_URL ?? "https://example.com").replace(/\/$/, "");
	return locales.flatMap((lang) =>
		["", "about/"].map((path) => ({ url: `${base}/${lang}/${path}` })),
	);
}
