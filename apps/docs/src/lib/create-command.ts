import { templates, type TemplateMeta } from "@/data/templates";
import { encodePreset, type PresetEnv, type PresetItem } from "@/lib/preset";

type PresetKind = PresetItem["kind"];

const KIND_ORDER: PresetKind[] = ["b", "f", "l"];

export type CommandProject = PresetItem & {
  uid?: string;
  templateId: string;
  title: TemplateMeta["title"];
  defaultName: string;
  name: string;
};

export function buildCreateCommand(input: {
  workspaceName: string;
  presetId: string;
  projectNames: string[];
}) {
  const workspaceName = input.workspaceName.trim() || "my-workspace";
  const names = input.projectNames.map((name) => name.trim()).filter(Boolean);
  const projectNames =
    names.length > 0 ? ` --project-names ${names.join(",")}` : "";
  return `one create ${workspaceName} --preset ${input.presetId}${projectNames} --yes -o json`;
}

export function projectsFromPresetId(presetId: string): CommandProject[] {
  const id = presetId.trim().replace(/^preset:/, "");
  const parts = id.split(".").slice(1);
  const counts = new Map<string, number>();
  const projects: CommandProject[] = [];

  for (const segment of parts) {
    const kind = segment[0] as PresetKind | "e";
    if (kind !== "b" && kind !== "f" && kind !== "l") continue;
    const templateCode = segment.slice(1, 3);
    const deployCode = kind === "l" ? undefined : segment[3];
    const containerCode = kind === "l" ? undefined : segment[4];
    const template = templates.find(
      (t) => t.presetKind === kind && t.code === templateCode,
    );
    if (!template) continue;

    const countKey = template.id;
    const occurrence = counts.get(countKey) ?? 0;
    counts.set(countKey, occurrence + 1);
    const defaultName =
      occurrence === 0
        ? template.defaultName
        : `${template.defaultName}-${occurrence + 1}`;

    projects.push({
      kind,
      tcode: template.code,
      dcode: deployCode,
      ccode: containerCode,
      templateId: template.id,
      title: template.title,
      defaultName,
      name: defaultName,
    });
  }

  return sortCommandProjects(projects);
}

export function envFromPresetId(presetId: string): "dotenv" | "infisical" {
  const id = presetId.trim().replace(/^preset:/, "");
  const envSegment = id.split(".").find((segment) => segment[0] === "e");
  return envSegment === "ei" ? "infisical" : "dotenv";
}

export function buildCustomPresetCommand(input: {
  workspaceName: string;
  env: "dotenv" | "infisical";
  projects: CommandProject[];
}) {
  const sorted = sortCommandProjects(input.projects);
  const envCode: PresetEnv = input.env === "infisical" ? "i" : "d";
  const presetId = encodePreset(
    sorted.map((project) => ({
      kind: project.kind,
      tcode: project.tcode,
      dcode: project.dcode,
      ccode: project.ccode,
    })),
    envCode,
  );
  return buildCreateCommand({
    workspaceName: input.workspaceName,
    presetId,
    projectNames: sorted.map((project) => project.name),
  });
}

export function sortCommandProjects<T extends PresetItem>(projects: T[]): T[] {
  return [...projects].sort((a, b) => {
    const kindDelta = KIND_ORDER.indexOf(a.kind) - KIND_ORDER.indexOf(b.kind);
    if (kindDelta !== 0) return kindDelta;
    if (a.tcode !== b.tcode) return compareAscii(a.tcode, b.tcode);
    if ((a.dcode ?? "") !== (b.dcode ?? "")) {
      return compareAscii(a.dcode ?? "", b.dcode ?? "");
    }
    return compareAscii(a.ccode ?? "", b.ccode ?? "");
  });
}

function compareAscii(a: string, b: string) {
  if (a < b) return -1;
  if (a > b) return 1;
  return 0;
}
