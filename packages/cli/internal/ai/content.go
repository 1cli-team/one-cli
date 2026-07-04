package ai

import (
	"fmt"
	"sort"
	"strings"
)

// RootAgentsContent is the initial canonical AGENTS.md body written by
// `one create` and back-filled by ai.Refresh for older workspaces.
func RootAgentsContent(workspaceName string) string {
	name := strings.TrimSpace(workspaceName)
	if name == "" {
		name = "One workspace"
	}
	return fmt.Sprintf("# %s Agent Guide\n\n", name) +
		"Scope: this file is the canonical agent entry point for this One workspace.\n\n" +
		"Always-on rules:\n" +
		"- Treat `one.manifest.json` as the source of truth for projects and domains.\n" +
		"- Use `one add` to add projects and let the CLI refresh generated agent docs.\n" +
		"- Read the route below, then open only the detail files relevant to the task.\n" +
		"- Do not hand-edit content between One CLI managed markers.\n\n" +
		"## On-demand Routes\n\n" +
		"<!-- one agents:index:start -->\n" +
		"*No project routes yet. Run* `one add <template-id> --name <name>` *to scaffold one.*\n" +
		"<!-- one agents:index:end -->\n\n" +
		"## Sub-projects\n\n" +
		"<!-- one subprojects:start -->\n" +
		"*No sub-projects yet. Run* `one add <template-id> --name <name>` *to scaffold one.*\n" +
		"<!-- one subprojects:end -->\n"
}

// ClaudePointerContent keeps Claude Code on the canonical AGENTS.md entry
// instead of maintaining a second copy of the same workspace guide.
func ClaudePointerContent() string {
	return "Follow ./AGENTS.md\n"
}

func ConventionsContent() string {
	return "# Workspace Conventions\n\n" +
		"This directory is generated from `one.manifest.json` by One CLI.\n\n" +
		"- Keep project identity, paths, templates, and domain selections in `one.manifest.json`.\n" +
		"- Add projects with `one add <template-id> --name <name>` so the manifest, infrastructure files, and agent docs stay in sync.\n" +
		"- Use the root `AGENTS.md` route table first, then open the matching project or ops guide on demand.\n" +
		"- Do not put credentials, tokens, private keys, or `.env*` values in agent-facing docs.\n" +
		"- If generated docs look stale, rerun the One CLI workflow that changes the manifest; do not invent a manual agent-doc refresh command.\n"
}

func DevOpsContent() string {
	return `# Dev Operations

Use ` + "`one dev`" + ` from the workspace root to start manifest-backed development processes.

- Project dev commands live at ` + "`projects[].domains.dev.command`" + `.
- A missing dev command means the project is not part of the dev supervisor.
- For one-off commands with project environment injection, use ` + "`one run`" + ` from the workspace root or a project directory.
`
}

func SecretsOpsContent() string {
	return "# Environment Operations\n\n" +
		"Use `one env` for workspace and project environment variables.\n\n" +
		"- `one env set` records declared keys without storing values in `one.manifest.json`.\n" +
		"- `one env get`, `one env list`, and `one env pull` read from the selected env backend.\n" +
		"- Do not commit `.env*` values, private keys, or provider tokens.\n"
}

func ContainerOpsContent(kinds []string) string {
	kinds = sortedStrings(kinds)
	var b strings.Builder
	b.WriteString("# Container Operations\n\n")
	b.WriteString("Use `one container` for projects with `projects[].domains.container` configured.\n\n")
	b.WriteString("- `one container info` shows build targets and selected container kinds.\n")
	b.WriteString("- `one container build <project>` builds a project image.\n")
	b.WriteString("- `one container push <project>` pushes a previously built image when a registry profile is configured.\n")
	if len(kinds) > 0 {
		b.WriteString("\nConfigured kinds:\n")
		for _, kind := range kinds {
			b.WriteString("- `" + kind + "`\n")
		}
	}
	return b.String()
}

func DeployOpsContent(kinds []string) string {
	kinds = sortedStrings(kinds)
	var b strings.Builder
	b.WriteString("# Deploy Operations\n\n")
	b.WriteString("Use `one deploy` for projects with `projects[].domains.deploy` configured.\n")
	if len(kinds) == 0 {
		return b.String()
	}
	b.WriteString("\nConfigured deploy kinds:\n")
	for _, kind := range kinds {
		b.WriteString("\n## " + kind + "\n\n")
		switch kind {
		case "kustomize":
			b.WriteString("- `one deploy` builds and applies the generated kustomize workload when deployment credentials are configured.\n")
			b.WriteString("- Kustomize files live under `kustomize/` and mirror the workspace environments.\n")
		case "vercel", "cloudflare", "edgeone":
			b.WriteString("- `one deploy -p <project>` runs the provider-specific static or web deployment flow.\n")
			b.WriteString("- Provider credentials should be configured through `one configure`, not stored in docs.\n")
		default:
			b.WriteString("- `one deploy -p <project>` runs the S3-compatible deployment flow for this backend.\n")
			b.WriteString("- Bucket and profile state come from `one.manifest.json` plus `one configure`.\n")
		}
	}
	return b.String()
}

func sortedStrings(in []string) []string {
	out := append([]string{}, in...)
	sort.Strings(out)
	return out
}
