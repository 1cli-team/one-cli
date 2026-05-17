import fs from "node:fs";
import path from "node:path";
import { locales, type Locale } from "@/i18n";

export type BlogPost = {
  slug: string;
  lang: Locale;
  title: string;
  description: string;
  date: string;
  author: string;
  tags: string[];
  readingTime: number;
  content: string;
  contentPath: string;
};

type Frontmatter = Record<string, string | string[]>;

const contentRoot = path.join(process.cwd(), "content", "blog");
const repoContentRoot = path.join(process.cwd(), "apps", "docs", "content", "blog");

// Module-level cache to avoid O(n²) build time when computing related posts
const blogPostsCache = new Map<Locale, BlogPost[]>();

export function getBlogPosts(lang: Locale) {
  if (blogPostsCache.has(lang)) {
    return blogPostsCache.get(lang)!;
  }

  const root = getBlogRoot();
  const dir = path.join(root, lang);
  if (!fs.existsSync(dir)) {
    blogPostsCache.set(lang, []);
    return [];
  }

  const posts = fs
    .readdirSync(dir)
    .filter((file) => file.endsWith(".md"))
    .map((file) => readBlogPostFromFile(lang, path.join(dir, file)))
    .sort((a, b) => b.date.localeCompare(a.date));

  blogPostsCache.set(lang, posts);
  return posts;
}

export function getAllBlogPosts() {
  return locales.flatMap((lang) => getBlogPosts(lang));
}

export function getBlogPost(lang: Locale, slug: string) {
  const filePath = path.join(getBlogRoot(), lang, `${slug}.md`);
  if (!fs.existsSync(filePath)) return null;
  return readBlogPostFromFile(lang, filePath);
}

export function formatBlogDate(lang: Locale, date: string) {
  return new Intl.DateTimeFormat(lang === "zh" ? "zh-CN" : "en-US", {
    year: "numeric",
    month: "short",
    day: "numeric",
  }).format(new Date(`${date}T00:00:00Z`));
}

function readBlogPostFromFile(lang: Locale, filePath: string): BlogPost {
  const raw = fs.readFileSync(filePath, "utf8");
  const slug = path.basename(filePath, ".md");
  const { frontmatter, content } = parseFrontmatter(raw, filePath);
  const title = readRequired(frontmatter, "title", filePath);
  const description =
    readOptional(frontmatter, "description") ?? firstParagraph(content);
  const date = readRequired(frontmatter, "date", filePath);
  const author = readOptional(frontmatter, "author") ?? "One CLI Team";
  const tags = readTags(frontmatter.tags);

  return {
    slug,
    lang,
    title,
    description,
    date,
    author,
    tags,
    readingTime: estimateReadingTime(content),
    content,
    contentPath: `content/blog/${lang}/${slug}.md`,
  };
}

function getBlogRoot() {
  return fs.existsSync(contentRoot) ? contentRoot : repoContentRoot;
}

function parseFrontmatter(raw: string, filePath: string) {
  const match = raw.match(/^---\n([\s\S]*?)\n---\n?([\s\S]*)$/);
  if (!match) {
    throw new Error(`Blog post is missing frontmatter: ${filePath}`);
  }

  const frontmatter: Frontmatter = {};
  for (const line of match[1].split("\n")) {
    const trimmed = line.trim();
    if (!trimmed) continue;
    const separator = trimmed.indexOf(":");
    if (separator === -1) continue;
    const key = trimmed.slice(0, separator).trim();
    const value = trimmed.slice(separator + 1).trim();
    frontmatter[key] = parseFrontmatterValue(value);
  }

  return {
    frontmatter,
    content: match[2].trim(),
  };
}

function parseFrontmatterValue(value: string) {
  if (value.startsWith("[") && value.endsWith("]")) {
    const rawItems = value.slice(1, -1).split(",");
    return rawItems.map((item) => stripQuotes(item.trim())).filter(Boolean);
  }

  return stripQuotes(value);
}

function stripQuotes(value: string) {
  return value.replace(/^["']|["']$/g, "");
}

function readRequired(frontmatter: Frontmatter, key: string, filePath: string) {
  const value = frontmatter[key];
  if (typeof value !== "string" || value.length === 0) {
    throw new Error(`Blog post ${filePath} is missing ${key}`);
  }
  return value;
}

function readOptional(frontmatter: Frontmatter, key: string) {
  const value = frontmatter[key];
  return typeof value === "string" && value.length > 0 ? value : undefined;
}

function readTags(value: string | string[] | undefined) {
  if (Array.isArray(value)) return value;
  if (typeof value === "string" && value.length > 0) return [value];
  return [];
}

function firstParagraph(content: string) {
  return (
    content
      .split(/\n{2,}/)
      .map((block) => block.trim())
      .find((block) => block && !block.startsWith("#")) ?? ""
  );
}

function estimateReadingTime(content: string) {
  const text = content.replace(/```[\s\S]*?```/g, "").replace(/\s+/g, "");
  return Math.max(3, Math.ceil(text.length / 500));
}
