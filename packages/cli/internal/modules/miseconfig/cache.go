package miseconfig

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/resources/bundled"
	"gopkg.in/yaml.v3"
)

func shellQuote(s string) string   { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
func windowsQuote(s string) string { return "\"" + strings.ReplaceAll(s, "\"", "\"\"") + "\"" }

// Only known deterministic template tasks receive generated cache declarations.
// Other tasks may opt in explicitly through the user's mise.toml.
func (p *Plan) configureCache(root string, project workspace.Project, op workspace.ProjectTask, t *Task) {
	outputs := []string{}
	if op.Name != "build" {
		return
	}
	switch project.TemplateID {
	case "react-spa", "astro-site", "starlight-docs", "ts-library", "nestjs-api":
		outputs = []string{"dist"}
	case "nextjs-app":
		outputs = []string{".next"}
	case "go-api":
		outputs = []string{"bin"}
	case "go-lib":
	default:
		return
	}
	if !p.templateCacheConfigUnchanged(project) {
		return
	}
	// Replaced scripts must supply their own cache contract.
	if project.Toolchain == "node" {
		raw, err := p.read(project.RelativeDir + "/package.json")
		if err != nil {
			return
		}
		var pkg struct {
			Scripts map[string]string `json:"scripts"`
		}
		if json.Unmarshal(raw, &pkg) != nil {
			return
		}
		allowed := map[string][]string{"react-spa": {"tsc -b && vite build", "vite build"}, "astro-site": {"pnpm astro check && astro build"}, "starlight-docs": {"pnpm astro check && astro build"}, "ts-library": {"tsdown"}, "nestjs-api": {"nest build"}, "nextjs-app": {"next build"}}
		ok := false
		for _, command := range allowed[project.TemplateID] {
			if pkg.Scripts[op.Name] == command {
				ok = true
			}
		}
		if !ok {
			return
		}
	} else if project.Toolchain == "go" {
		raw, err := p.read(project.RelativeDir + "/Taskfile.yml")
		if err != nil {
			return
		}
		var doc struct {
			Tasks map[string]yaml.Node `yaml:"tasks"`
		}
		if yaml.Unmarshal(raw, &doc) != nil {
			return
		}
		var build struct {
			Cmds []string `yaml:"cmds"`
		}
		node := doc.Tasks["build"]
		if node.Decode(&build) != nil {
			return
		}
		commands := build.Cmds
		expected := "go build {{.CLI_ARGS}} ./..."
		if project.TemplateID == "go-api" {
			expected = "go build {{.CLI_ARGS}} -o bin/server{{.GOEXE}} ./cmd/server"
		}
		if len(commands) != 1 || commands[0] != expected {
			return
		}
	}
	relative, _ := filepath.Rel(project.TargetDir, root)
	relative = filepath.ToSlash(relative)
	t.Sources = []string{"**/*", "!node_modules/**", "!.git/**", "!.mise/**", "!.cache/**", "!coverage/**", "!dist/**", "!bin/**", "!.next/**", "!.astro/**", relative + "/one.manifest.json", relative + "/package.json", relative + "/pnpm-lock.yaml", relative + "/pnpm-workspace.yaml", relative + "/go.work", relative + "/go.work.sum"}
	for _, input := range op.Inputs {
		rel, _ := filepath.Rel(project.TargetDir, filepath.Join(root, input))
		t.Sources = append(t.Sources, filepath.ToSlash(rel))
	}
	t.Outputs = &outputs
	t.Cache = &Cache{Enabled: true, Env: []string{"NODE_ENV", "CI", "GOOS", "GOARCH", "CGO_ENABLED", "GOFLAGS", "GOTOOLCHAIN", "CC", "CXX", "CFLAGS", "CXXFLAGS", "LDFLAGS", "CGO_CFLAGS", "CGO_CPPFLAGS", "CGO_CXXFLAGS", "CGO_LDFLAGS", "GOPATH", "GOWORK"}}
}

// Build configuration can redirect output through arbitrary code. Only infer an
// output contract while it matches the bundled scaffold. Customized projects
// can declare their actual contract in mise.toml without executing config here.
func (p *Plan) templateCacheConfigUnchanged(project workspace.Project) bool {
	files := map[string][]string{
		"react-spa":      {"vite.config.ts", "tsconfig.json", "tsconfig.app.json", "tsconfig.node.json"},
		"astro-site":     {"astro.config.mjs", "tsconfig.json"},
		"starlight-docs": {"astro.config.mjs", "tsconfig.json"},
		"ts-library":     {"tsdown.config.ts", "tsconfig.json"},
		"nestjs-api":     {"nest-cli.json", "tsconfig.json", "tsconfig.build.json"},
		"nextjs-app":     {"next.config.ts", "tsconfig.json"},
		"go-api":         {"Taskfile.yml"},
		"go-lib":         {"Taskfile.yml"},
	}[project.TemplateID]
	if len(files) == 0 {
		return false
	}
	for _, file := range files {
		expected, err := bundled.TemplatesFS.ReadFile(bundled.TemplatesRoot + "/" + project.TemplateID + "/" + file)
		if err != nil {
			return false
		}
		actual, err := p.readOptional(project.RelativeDir + "/" + file)
		if err != nil || !bytes.Equal(bytes.ReplaceAll(actual, []byte("\r\n"), []byte("\n")), bytes.ReplaceAll(expected, []byte("\r\n"), []byte("\n"))) {
			return false
		}
		// A second config in another supported format may take precedence.
		if strings.Contains(file, ".config.") {
			base := strings.TrimSuffix(file, filepath.Ext(file))
			for _, extension := range []string{".js", ".mjs", ".cjs", ".ts", ".mts", ".cts"} {
				if candidate := base + extension; candidate != file {
					raw, err := p.readOptional(project.RelativeDir + "/" + candidate)
					if err != nil || raw != nil {
						return false
					}
				}
			}
		}
	}
	return true
}

// nativeCommand keeps normal script names readable and quotes user-defined names.
func nativeCommand(argv []string, windows bool) string {
	parts := make([]string, len(argv))
	for i, arg := range argv {
		if arg != "" && strings.IndexFunc(arg, func(r rune) bool {
			return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("_./:-", r))
		}) < 0 {
			parts[i] = arg
		} else if windows {
			parts[i] = windowsQuote(arg)
		} else {
			parts[i] = shellQuote(arg)
		}
	}
	return strings.Join(parts, " ")
}
