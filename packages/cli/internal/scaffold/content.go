package scaffold

import (
	agentdocs "github.com/torchstellar-team/one-cli/packages/cli/internal/ai"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

// These are the verbatim file contents the scaffolder writes.

const pnpmWorkspaceContent = `packages:
  - "apps/*"
  - "services/*"
  - "packages/*"
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

const commitlintConfigContent = `module.exports = {
  extends: ['@commitlint/config-conventional']
};
`

const huskyPreCommitContent = `#!/usr/bin/env sh
echo "pre-commit hook: add checks for this repository."
`

const huskyCommitMsgContent = `#!/usr/bin/env sh
npx --no -- commitlint --edit "$1"
`

const dockerComposeContent = `services:
  # one-cli:services:start
  # one-cli:services:end
`

const k8sDeploymentContent = `# one-cli:resources:start
# one-cli:resources:end
`

// buildRootAgentsMd renders the canonical workspace AGENTS.md skeleton.
func buildRootAgentsMd(projectName string) string {
	return agentdocs.RootAgentsContent(projectName)
}

// buildClaudeMdPointer renders the Claude Code pointer to the canonical
// AGENTS.md entry.
func buildClaudeMdPointer() string {
	return agentdocs.ClaudePointerContent()
}

func buildAgentsConventionsMd() string {
	return agentdocs.ConventionsContent()
}

// changesetConfig is the .changeset/config.json scaffolders write at
// create time. Encoded as orderedJSON so json.MarshalIndent preserves
// the field order below.
var changesetConfig = orderedJSON{
	{Key: "$schema", Value: "https://unpkg.com/@changesets/config@3.1.1/schema.json"},
	{Key: "changelog", Value: "@changesets/cli/changelog"},
	{Key: "commit", Value: false},
	{Key: "fixed", Value: []any{}},
	{Key: "linked", Value: []any{}},
	{Key: "access", Value: "restricted"},
	{Key: "baseBranch", Value: "main"},
	{Key: "updateInternalDependencies", Value: "patch"},
	{Key: "ignore", Value: []any{}},
}

// Package manager spec strings shipped in the workspace-root
// package.json.
const (
	packageManagerName = "pnpm"
	packageManagerSpec = "pnpm@10.14.0"
)

// buildPackageJSON returns the workspace root package.json. Workspace-scope
// One configuration moved into one.manifest.json in v2 — this file is now
// a plain pnpm root with no `one` block.
func buildPackageJSON(name string) orderedJSON {
	return orderedJSON{
		{Key: "name", Value: name},
		{Key: "private", Value: true},
		{Key: "version", Value: "0.0.0"},
		{Key: "packageManager", Value: packageManagerSpec},
		{Key: "scripts", Value: orderedJSON{
			{Key: "prepare", Value: "husky"},
			{Key: "changeset", Value: "changeset"},
		}},
		{Key: "devDependencies", Value: orderedJSON{
			{Key: "@changesets/cli", Value: "latest"},
			{Key: "@commitlint/cli", Value: "latest"},
			{Key: "@commitlint/config-conventional", Value: "latest"},
			{Key: "husky", Value: "latest"},
		}},
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
