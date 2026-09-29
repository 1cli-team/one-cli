import registry from "../../../../packages/templates/registry.json";
import localizedCopy from "./template-copy.json";

export type TemplateKind = "frontend" | "backend" | "library";
export type LocalizedText = { zh: string; en: string };
export type TemplateMeta = {
  id: string;
  kind: TemplateKind;
  title: LocalizedText;
  tagline: LocalizedText;
  toolchain: "node" | "go" | "none";
  directory: "apps" | "services" | "packages";
  tags: string[];
  cover: string;
  overview: LocalizedText;
  features: LocalizedText[];
};

const descriptions: Record<string, {
  name: LocalizedText;
  description: LocalizedText;
  overview: LocalizedText;
  features: LocalizedText[];
}> = localizedCopy;
const directories = { frontend: "apps", backend: "services", library: "packages" } as const;
// Keep the original gallery illustrations and their display order.
const covers: Record<string, string> = {
  "expo-mobile": "/examples/mobile-starter/cover.png",
  "electron-app": "/examples/desktop-starter/cover.png",
  "astro-site": "/examples/landing-starter/cover.png",
  "starlight-docs": "/examples/docs-starter/cover.png",
  "nextjs-app": "/examples/consumer-starter/cover.png",
  "react-spa": "/examples/admin-starter/cover.png",
  "nestjs-api": "/templates/nestjs-api/cover.png",
  "go-api": "/templates/go-api/cover.png",
  "ts-library": "/templates/ts-library/cover.png",
  "go-lib": "/templates/go-lib/cover.png",
  "empty-app": "/templates/empty-app/cover.png",
  "empty-service": "/templates/empty-service/cover.png",
  "empty-library": "/templates/empty-library/cover.png",
};
const coverOrder = Object.keys(covers);
const displayOrder = (id: string) => {
  const index = coverOrder.indexOf(id);
  return index === -1 ? coverOrder.length : index;
};
const ids = new Set(registry.templates.map((entry) => entry.id));
if (ids.size !== registry.templates.length || Object.keys(descriptions).some((id) => !ids.has(id))) {
  throw new Error("Template catalog IDs must match packages/templates/registry.json");
}

// Empty scaffolds remain available in the CLI but are not part of the website catalog.
const hiddenTemplateIds = new Set(["empty-app", "empty-service", "empty-library"]);

// The registry owns technical metadata. Every displayed template must have
// complete website copy in both languages; missing copy fails the build.
export const templates: TemplateMeta[] = [...registry.templates]
  .filter((entry) => !hiddenTemplateIds.has(entry.id))
  .sort((a, b) => displayOrder(a.id) - displayOrder(b.id))
  .map((entry) => {
    const copy = descriptions[entry.id];
    for (const lang of ["zh", "en"] as const) {
      if (
        !copy?.name[lang]?.trim() ||
        !copy?.description[lang]?.trim() ||
        !copy?.overview[lang]?.trim() ||
        !copy?.features.length ||
        copy.features.some((feature) => !feature[lang]?.trim())
      ) {
        throw new Error(`Missing ${lang} template description: ${entry.id}`);
      }
    }
    if (!(entry.category in directories) || !["node", "go", "none"].includes(entry.toolchain)) {
      throw new Error(`Unsupported template metadata: ${entry.id}`);
    }
    const kind = entry.category as TemplateKind;
    return {
      id: entry.id,
      kind,
      title: copy.name,
      tagline: copy.description,
      toolchain: entry.toolchain as TemplateMeta["toolchain"],
      directory: directories[kind],
      tags: entry.tags,
      cover: covers[entry.id] ?? "/examples/_placeholder.svg",
      overview: copy.overview,
      features: copy.features,
    };
  });

export function getTemplateById(id: string) {
  return templates.find((template) => template.id === id);
}
