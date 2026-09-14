// Package hooks owns workspace-wide hk configuration and local Git launchers.
package hooks

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/fsutil"
)

const configHeader = "// Managed by One CLI hooks/v1; sha256="
const userConfig = "// Customize workspace checks here; One refreshes only .config/one/hk.pkl.\namends \".config/one/hk.pkl\"\n"

// PlanFiles is static and participates in creation's workspace transaction.
// Existing hk.pkl overrides are never rewritten.
func PlanFiles(p *fsutil.FilePlan, m *workspace.Manifest) error {
	before, err := p.Read(workspace.HooksConfigFilename)
	if err != nil {
		return err
	}
	if before != nil {
		if err := validateManaged(workspace.HooksConfigFilename, before, configHeader); err != nil {
			return err
		}
	}
	rootConfig, err := p.Read("hk.pkl")
	if err != nil {
		return err
	}
	if rootConfig != nil && before == nil && !strings.Contains(string(rootConfig), `amends ".config/one/hk.pkl"`) {
		return conflict("hk.pkl", "an existing hk configuration is present; integrate it with .config/one/hk.pkl before configuring One hooks")
	}
	if rootConfig == nil {
		if err := p.Set("hk.pkl", []byte(userConfig), 0o644); err != nil {
			return err
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "amends %s\nimport %s\n\n", pklQuote(schemaURL("Config.pkl")), pklQuote(schemaURL("Builtins.pkl")))
	fmt.Fprintf(&b, "min_hk_version = %s\n\n", pklQuote(workspace.HKVersion))
	b.WriteString("local one = read?(\"env:ONE_BINARY_PATH\") ?? \"one\"\n\n")
	b.WriteString("// Shared by local checks, explicit fixes, and the pre-commit hook.\nsteps {\n")
	projects := append([]workspace.ManifestProject(nil), m.Projects...)
	sort.Slice(projects, func(i, j int) bool { return projects[i].Name < projects[j].Name })
	for _, project := range projects {
		dir := filepath.ToSlash(filepath.Clean(project.RelativeDir))
		if !workspace.IsValidProjectName(project.Name) || dir == "." {
			return conflict(project.Name, "invalid project name or directory")
		}
		if err := fsutil.SafeWritePath(p.Root, filepath.Join(p.Root, dir, "hk.pkl")); err != nil {
			return err
		}
		switch project.Toolchain {
		case "go":
			fmt.Fprintf(&b, "  [%s] {\n    dir = %s\n    glob = \"**/*.go\"\n    exclude = List(\"**/vendor/**\")\n    check = new Command { argv = List(\"mise\", \"exec\", \"--\", one, \"__hook-gofmt\", \"--\", \"{{files}}\") }\n    fix = new Command { argv = List(\"mise\", \"exec\", \"--\", \"gofmt\", \"-w\", \"--\", \"{{files}}\") }\n  }\n", pklQuote(project.Name+":format"), pklQuote(dir))
		case "node":
			raw, err := p.Read(dir + "/package.json")
			if err != nil {
				return err
			}
			var pkg struct {
				Scripts         map[string]string `json:"scripts"`
				Dependencies    map[string]string `json:"dependencies"`
				DevDependencies map[string]string `json:"devDependencies"`
			}
			if err := json.Unmarshal(raw, &pkg); err != nil {
				return fmt.Errorf("%s/package.json: %w", dir, err)
			}
			pm := project.PackageManager
			if pm == "" {
				pm = "pnpm"
			}
			prefix, err := packageExec(pm)
			if err != nil {
				return err
			}
			for _, tool := range []struct{ name, builtin, operation string }{{"oxlint", "ox_lint", "lint"}, {"oxfmt", "oxfmt", "format"}} {
				if pkg.Dependencies[tool.name] != "" || pkg.DevDependencies[tool.name] != "" {
					fmt.Fprintf(&b, "  [%s] = (Builtins.%s) {\n    dir = %s\n    prefix = %s\n    exclude = List(\"**/.mise/**\", \"**/node_modules/**\", \"**/dist/**\", \"**/pnpm-lock.yaml\", \"**/package-lock.json\")\n", pklQuote(project.Name+":"+tool.operation), tool.builtin, pklQuote(dir), pklList(append([]string{"mise", "exec", "--"}, prefix...)))
					if tool.name == "oxfmt" {
						// hk 2.0 resolves --list-different output against the root,
						// while oxfmt reports paths relative to this step's dir.
						// Use its ordinary read-only check before fixing instead.
						b.WriteString("    check_list_files = null\n")
					}
					b.WriteString("  }\n")
					continue
				}
				// Imported projects may use other tools. Retain their explicit
				// script contract, but never guess that a bare format script is read-only.
				script, fix := "lint", "lint:fix"
				if tool.operation == "format" {
					script, fix = "format:check", "format:fix"
				}
				if pkg.Scripts[script] == "" {
					continue
				}
				fmt.Fprintf(&b, "  [%s] {\n    dir = %s\n    glob = \"**/*\"\n    exclusive = true\n    check = new Command { argv = %s }\n", pklQuote(project.Name+":"+tool.operation), pklQuote(dir), pklList([]string{"mise", "exec", "--", pm, "run", script}))
				if pkg.Scripts[fix] != "" {
					fmt.Fprintf(&b, "    fix = new Command { argv = %s }\n", pklList([]string{"mise", "exec", "--", pm, "run", fix}))
				}
				b.WriteString("  }\n")
			}
		}
	}
	b.WriteString("}\n\nhooks {\n  [\"pre-commit\"] {\n    fix = false\n    stage = false\n    stash = \"git\"\n  }\n  [\"commit-msg\"] {\n    steps {\n      [\"conventional-commit\"] {\n        check = new Command { argv = List(\"hk\", \"util\", \"check-conventional-commit\", \"{{commit_msg_file}}\") }\n      }\n    }\n  }\n}\n")
	body := []byte(b.String())
	after := []byte(fmt.Sprintf("%s%x\n// Customize hk.pkl; refresh generated defaults with one configure hooks.\n%s", configHeader, sha256.Sum256(body), body))
	return p.Set(workspace.HooksConfigFilename, after, 0o644)
}

func schemaURL(name string) string {
	return "package://github.com/jdx/hk/releases/download/v" + workspace.HKVersion + "/hk@" + workspace.HKVersion + "#/" + name
}

func pklQuote(value string) string { return strconv.Quote(value) }

func pklList(values []string) string {
	quoted := make([]string, len(values))
	for i, value := range values {
		quoted[i] = pklQuote(value)
	}
	return "List(" + strings.Join(quoted, ", ") + ")"
}

func packageExec(manager string) ([]string, error) {
	switch manager {
	case "pnpm", "yarn":
		return []string{manager, "exec"}, nil
	case "npm":
		return []string{"npm", "exec", "--offline", "--"}, nil
	case "bun":
		return []string{"bun", "run"}, nil
	default:
		return nil, fmt.Errorf("unsupported hook package manager %q", manager)
	}
}

func validateManaged(path string, raw []byte, header string) error {
	lines := bytes.SplitN(raw, []byte("\n"), 3)
	if len(lines) != 3 || string(lines[0]) != fmt.Sprintf("%s%x", header, sha256.Sum256(lines[2])) {
		return conflict(path, "file is user-owned or has been modified; preserve it and resolve the conflict before regenerating")
	}
	return nil
}

func conflict(path, reason string) error {
	return cliErrors.New(cliErrors.HOOKS_CONFIG_CONFLICT, fmt.Sprintf("%s: %s", path, reason)).WithContext(map[string]any{"path": path})
}
