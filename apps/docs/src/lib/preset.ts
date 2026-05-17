// Client-side One CLI preset id encoder.
// Mirrors packages/cli/internal/preset/spec.go + codes.go (v1).
// Grammar: 1[.<kind><tcode>[<dcode>[<ccode>]]]+[.e<envCode>]
//   kind: 'f' (frontend) | 'b' (backend) | 'l' (library)
//   tcode: 2-char [a-z0-9] template code
//   dcode: 1-char deploy code (optional; empty = template default)
//   ccode: 1-char container code (optional; only meaningful with kustomize)
//   envCode: 1-char env code (optional; empty = workspace default dotenv)
// Canonical ordering: items sorted by kind (b < f < l).

export type PresetKind = "f" | "b" | "l";
export type PresetEnv = "d" | "i";

export type PresetItem = {
  kind: PresetKind;
  tcode: string;
  dcode?: string;
  ccode?: string;
};

const KIND_ORDER: PresetKind[] = ["b", "f", "l"];

export function encodePreset(
  items: PresetItem[],
  envCode?: PresetEnv | "",
): string {
  if (items.length === 0) return "";
  const sorted = [...items].sort(
    (a, b) => KIND_ORDER.indexOf(a.kind) - KIND_ORDER.indexOf(b.kind),
  );
  const segs = sorted.map(
    (it) => `${it.kind}${it.tcode}${it.dcode ?? ""}${it.ccode ?? ""}`,
  );
  const out = ["1", ...segs];
  if (envCode) out.push(`e${envCode}`);
  return out.join(".");
}
