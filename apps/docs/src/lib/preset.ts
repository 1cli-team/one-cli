// Client-side One CLI preset id encoder.
// Mirrors packages/cli/internal/modules/preset/spec.go + codes.go (v1).
// Grammar: 1[.<kind><tcode>]+[.e<envCode>]
//   kind: 'f' (frontend) | 'b' (backend) | 'l' (library)
//   tcode: 2-char [a-z0-9] template code
//   envCode: 1-char env code (optional; empty = Infisical)
// Canonical ordering: items sorted by kind (b < f < l), then template code.

export type PresetKind = "f" | "b" | "l";
export type PresetEnv = "i";

export type PresetItem = {
  kind: PresetKind;
  tcode: string;
};

const KIND_ORDER: PresetKind[] = ["b", "f", "l"];

export function encodePreset(
  items: PresetItem[],
  envCode?: PresetEnv | "",
): string {
  if (items.length === 0) return "";
  const sorted = [...items].sort(
    (a, b) => KIND_ORDER.indexOf(a.kind) - KIND_ORDER.indexOf(b.kind) || a.tcode.localeCompare(b.tcode),
  );
  const segs = sorted.map(
    (it) => `${it.kind}${it.tcode}`,
  );
  const out = ["1", ...segs];
  if (envCode) out.push(`e${envCode}`);
  return out.join(".");
}
