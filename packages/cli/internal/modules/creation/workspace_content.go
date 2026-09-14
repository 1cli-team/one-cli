package creation

import (
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
)

// These are the verbatim file contents the scaffolder writes.

const pnpmWorkspaceContent = `packages:
  - "apps/*"
  - "services/*"
  - "packages/*"

# Native build steps used by the bundled templates.
allowBuilds:
  '@parcel/watcher': true
  '@scarf/scarf': false
  '@swc/core': true
  electron: true
  esbuild: true
  unrs-resolver: true
`

const gitignoreContent = `# dependencies
node_modules

# build output
dist
coverage

# environment
.env
.env.local

# secrets — private keys must NEVER be committed
# (the .secrets/.gitignore inside the dir is the primary defense; this is
# a belt-and-suspenders entry for the workspace root)
.secrets/keys/

# misc
.DS_Store
`

// Package manager spec strings shipped in the workspace-root
// package.json.
const packageManagerSpec = "pnpm@12.3.4"

// buildPackageJSON returns the workspace root package.json. Workspace-scope
// One configuration moved into one.manifest.json in v2 — this file is now
// a plain pnpm root with no `one` block.
func buildPackageJSON(name string) orderedJSON {
	return orderedJSON{
		{Key: "name", Value: name},
		{Key: "private", Value: true},
		{Key: "version", Value: "0.0.0"},
		{Key: "packageManager", Value: packageManagerSpec},
		{Key: "engines", Value: orderedJSON{{Key: "node", Value: "^24.15.0 || >=26.0.0"}}},
	}
}

// emptyManifest is the freshly-stamped one.manifest.json. Carries
// only the workspace identity (workspace.id + workspace.name) and an
// empty projects array; backend selections (env / deploy / container)
// land later via `env init`, `one add`, etc.
func emptyManifest(projectName string) orderedJSON {
	return orderedJSON{
		{Key: "version", Value: workspace.ManifestVersion},
		{Key: "workspace", Value: orderedJSON{
			{Key: "id", Value: workspace.GenerateProjectID(projectName)},
			{Key: "name", Value: projectName},
		}},
		{Key: "projects", Value: []any{}},
	}
}
