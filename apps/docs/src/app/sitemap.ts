import type { MetadataRoute } from "next";
import { alternateDocsLanguages } from "@/i18n";
import { siteUrl } from "@/lib/seo";
import { source } from "@/lib/source";

export const dynamic = "force-static";

export default function sitemap(): MetadataRoute.Sitemap {
  const docsEntries = source.getLanguages().flatMap(({ language, pages }) =>
    pages.map((page) => ({
      url: absolute(page.url),
      lastModified: new Date(),
      alternates: {
        languages: Object.fromEntries(
          Object.entries(alternateDocsLanguages(page.slugs)).map(
            ([locale, href]) => [locale, absolute(href)],
          ),
        ),
      },
    })),
  );
  return [
    {
      url: absolute("/"),
      lastModified: new Date(),
      alternates: {
        languages: homeAlternates(),
      },
    },
    {
      url: absolute("/zh/"),
      lastModified: new Date(),
      alternates: {
        languages: homeAlternates(),
      },
    },
    {
      url: absolute("/en/"),
      lastModified: new Date(),
      alternates: {
        languages: homeAlternates(),
      },
    },
    ...docsEntries,
  ];
}

function absolute(path: string) {
  return new URL(path, siteUrl).toString();
}

function homeAlternates() {
  return {
    "zh-Hans": absolute("/zh/"),
    en: absolute("/en/"),
    "x-default": absolute("/zh/"),
  };
}
