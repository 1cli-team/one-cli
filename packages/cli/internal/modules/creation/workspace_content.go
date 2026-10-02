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
  # The Electron template uses NSIS, not the optional Squirrel installer.
  electron-winstaller: false
  esbuild: true
  unrs-resolver: true
`

const gitignoreContent = `/.one/
.one-run-*/

# dependencies
node_modules

# build output
dist
coverage

# environment
.env
.env.*
!.env.example

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
// One configuration moved into one.manifest.toml in v2 — this file is now
// a plain pnpm root with no `one` block.
func buildPackageJSON(name string) orderedJSON {
	return orderedJSON{
		{Key: "name", Value: name},
		{Key: "version", Value: "0.0.0"},
		{Key: "private", Value: true},
		{Key: "engines", Value: orderedJSON{{Key: "node", Value: "^24.15.0 || >=26.0.0"}}},
		{Key: "packageManager", Value: packageManagerSpec},
	}
}

// emptyManifest is the freshly-stamped one.manifest.toml. Carries
// only the workspace identity (workspace.id + workspace.name) and an
// empty projects array; environment and project configuration are added later.
func emptyManifest(projectName string) *workspace.Manifest {
	return &workspace.Manifest{Version: workspace.ManifestVersion, Workspace: &workspace.ManifestWorkspace{ID: workspace.GenerateProjectID(projectName), Name: projectName}, Projects: []workspace.ManifestProject{}}
}
