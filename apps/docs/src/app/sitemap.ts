import type { MetadataRoute } from "next";
import {
  alternateBlogLanguages,
  alternateDocsLanguages,
  localizedBlogPath,
} from "@/i18n";
import { getAllBlogPosts } from "@/lib/blog";
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
  const blogEntries = getAllBlogPosts().map((post) => ({
    url: absolute(localizedBlogPath(post.lang, [post.slug])),
    lastModified: new Date(`${post.date}T00:00:00Z`),
    alternates: {
      languages: Object.fromEntries(
        Object.entries(alternateBlogLanguages([post.slug])).map(
          ([locale, href]) => [locale, absolute(href)],
        ),
      ),
    },
  }));

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
    {
      url: absolute("/zh/templates/"),
      lastModified: new Date(),
      alternates: {
        languages: {
          "zh-Hans": absolute("/zh/templates/"),
          en: absolute("/en/templates/"),
          "x-default": absolute("/zh/templates/"),
        },
      },
    },
    {
      url: absolute("/en/templates/"),
      lastModified: new Date(),
      alternates: {
        languages: {
          "zh-Hans": absolute("/zh/templates/"),
          en: absolute("/en/templates/"),
          "x-default": absolute("/zh/templates/"),
        },
      },
    },
    {
      url: absolute("/zh/blog/"),
      lastModified: new Date(),
      alternates: {
        languages: Object.fromEntries(
          Object.entries(alternateBlogLanguages()).map(([locale, href]) => [
            locale,
            absolute(href),
          ]),
        ),
      },
    },
    {
      url: absolute("/en/blog/"),
      lastModified: new Date(),
      alternates: {
        languages: Object.fromEntries(
          Object.entries(alternateBlogLanguages()).map(([locale, href]) => [
            locale,
            absolute(href),
          ]),
        ),
      },
    },
    ...blogEntries,
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
