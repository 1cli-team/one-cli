const fs = require('node:fs');
const path = require('node:path');
const { pathToFileURL } = require('node:url');

async function loadSkills(version) {
  // npm exec exposes the requested package through PATH, rather than through
  // the project's module resolution. Resolve it there and verify the pin.
  for (const directory of (process.env.PATH || '').split(path.delimiter)) {
    if (!directory) continue;
    let manifest;
    try {
      manifest = require.resolve('skills/package.json', { paths: [directory] });
    } catch {
      continue;
    }
    if (JSON.parse(fs.readFileSync(manifest, 'utf8')).version !== version) continue;
    return import(pathToFileURL(require.resolve('skills', { paths: [directory] })).href);
  }
  throw Object.assign(new Error(`Cannot resolve skills@${version}`), { code: 'ONE_SKILLS_MODULE_UNAVAILABLE' });
}

async function main(request) {
  const api = await loadSkills(request.version);
  const cwd = process.cwd();
  if (request.action === 'inspect') {
    const detected = new Set((await api.detectAgents({ cwd })).agents.map(agent => agent.id));
    const installed = await api.listSkills({ cwd, scope: 'all' });
    return {
      agents: api.getSupportedAgents().map(agent => ({ ...agent, detected: detected.has(agent.id) })),
      installed: installed.skills.map(skill => ({ name: skill.name, scope: skill.scope })),
    };
  }
  if (request.action !== 'install') throw Object.assign(new Error('Unknown action'), { code: 'ONE_SKILLS_REQUEST_INVALID' });

  const { group, options } = request;
  const targets = [];
  const warnings = [];
  let names = group.names;
  if (request.skipExisting) {
    const installed = await api.listSkills({ cwd, scope: options.scope });
    const existing = new Set(installed.skills.map(skill => skill.name));
    for (const skill of names.filter(name => existing.has(name))) {
      for (const agent of group.agents) {
        targets.push({ skill, agent, source: group.source, status: 'skipped' });
      }
    }
    names = names.filter(name => !existing.has(name));
  }
  if (names.length === 0) return { targets, warnings };

  let result;
  try {
    result = await api.installSkills(group.source, {
      cwd,
      skills: names,
      agents: group.agents,
      scope: options.scope,
      mode: options.mode,
      ...(group.ref ? { ref: group.ref } : {}),
      onWarning: warning => warnings.push(warning.message),
    });
  } catch (error) {
    // Preserve any pre-existing targets even when the remaining source fails.
    for (const skill of names) for (const agent of group.agents) {
      targets.push({ skill, agent, source: group.source, ref: group.ref,
        status: 'failed', code: error?.code, message: error?.message || String(error) });
    }
    return { targets, warnings };
  }
  const target = (value, status) => ({
    ...value,
    source: group.source,
    ref: result.source.commit || group.ref,
    status,
  });
  targets.push(...result.installed.map(value => target(value, 'installed')));
  targets.push(...result.failed.map(value => target(value, 'failed')));
  targets.push(...result.skipped.map(value => target({
    ...value, code: 'SKILLS_AGENT_DIRECTORY_MISSING', message: value.reason,
  }, 'failed')));
  return { targets, warnings };
}

(async () => {
  try {
    let raw = '';
    for await (const chunk of process.stdin) raw += chunk;
    process.stdout.write(JSON.stringify({ result: await main(JSON.parse(raw)) }));
  } catch (error) {
    process.stdout.write(JSON.stringify({ error: error?.message || String(error), code: error?.code }));
    process.exitCode = 1;
  }
})();
