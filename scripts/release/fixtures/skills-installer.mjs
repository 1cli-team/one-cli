#!/usr/bin/env node
import { createHash } from "node:crypto";
import {
  appendFileSync,
  existsSync,
  mkdirSync,
  readFileSync,
  readdirSync,
  writeFileSync,
} from "node:fs";
import { join, relative } from "node:path";

const args = process.argv.slice(2);
appendFileSync(process.env.ONE_SKILLS_SMOKE_CALLS, JSON.stringify(args) + "\n");
const lock = existsSync("skills-lock.json")
  ? JSON.parse(readFileSync("skills-lock.json", "utf8"))
  : { version: 1, skills: {} };
const [source, ref] = args[args.indexOf("add") + 1].split("#");

function files(directory, base = directory) {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const path = join(directory, entry.name);
    return entry.isDirectory()
      ? files(path, base)
      : [{ path: relative(base, path), raw: readFileSync(path) }];
  });
}

for (const name of args.slice(args.indexOf("--skill") + 1, args.indexOf("--agent"))) {
  const directory = join(".agents/skills", name);
  mkdirSync(directory, { recursive: true });
  const path = join(directory, "SKILL.md");
  if (!existsSync(path))
    writeFileSync(path, `---\nname: ${name}\ndescription: Release smoke fixture\n---\n`);
  const hash = createHash("sha256");
  for (const file of files(directory).sort((a, b) => a.path.localeCompare(b.path))) {
    hash.update(file.path);
    hash.update(file.raw);
  }
  lock.skills[name] = {
    source,
    sourceType: source.startsWith(".") ? "local" : "github",
    ...(ref ? { ref } : {}),
    computedHash: hash.digest("hex"),
  };
}
writeFileSync("skills-lock.json", JSON.stringify(lock, null, 2) + "\n");
