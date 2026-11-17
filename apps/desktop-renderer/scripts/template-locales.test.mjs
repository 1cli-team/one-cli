import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { createInstance } from "i18next";

const readJSON = (path) => JSON.parse(readFileSync(new URL(path, import.meta.url), "utf8"));
const registry = readJSON("../../../packages/templates/registry.json");
const resources = {
  "en-US": { translation: readJSON("../src/locales/en-US.json") },
  "zh-CN": { translation: readJSON("../src/locales/zh-CN.json") },
};

test("built-in template labels follow Desktop language changes without refetching", async () => {
  const i18n = createInstance();
  await i18n.init({ resources, lng: "en-US", fallbackLng: "en-US" });
  for (const language of ["en-US", "zh-CN", "en-US"]) {
    await i18n.changeLanguage(language);
    for (const { id } of registry.templates) {
      const labels = resources[language].translation.projectCreate.templates[id];
      assert.ok(labels, `Missing ${language} Desktop labels for ${id}`);
      for (const field of ["name", "description"]) {
        assert.ok(labels[field]?.trim());
        assert.equal(
          i18n.t(`projectCreate.templates.${id}.${field}`, { defaultValue: "stale API label" }),
          labels[field],
        );
      }
    }
  }
});

test("desktop dictionaries contain matching, nonempty messages and placeholders", () => {
  const flatten = (value, prefix = "") =>
    Object.fromEntries(
      Object.entries(value).flatMap(([key, item]) => {
        const path = prefix ? `${prefix}.${key}` : key;
        return typeof item === "string" ? [[path, item]] : Object.entries(flatten(item, path));
      }),
    );
  const en = flatten(resources["en-US"].translation);
  const zh = flatten(resources["zh-CN"].translation);
  assert.deepEqual(Object.keys(en).sort(), Object.keys(zh).sort());
  const placeholders = (value) =>
    [...value.matchAll(/{{\s*([^}]+?)\s*}}/g)].map((match) => match[1]).sort();
  for (const key of Object.keys(en)) {
    assert.ok(en[key].trim() && zh[key].trim(), `Empty translation: ${key}`);
    assert.deepEqual(placeholders(en[key]), placeholders(zh[key]), `Placeholder mismatch: ${key}`);
  }
});
