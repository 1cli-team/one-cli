package ai

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

const (
	agentsRootDir     = ".one/agents"
	agentsProjectsDir = ".one/agents/projects"
	agentsOpsDir      = ".one/agents/ops"
)

type route struct {
	Label string
	Path  string
}

func sortedManifestProjects(m *workspace.Manifest) []workspace.ManifestProject {
	if m == nil {
		return nil
	}
	out := append([]workspace.ManifestProject{}, m.Projects...)
	sort.Slice(out, func(i, j int) bool {
		return out[i].RelativeDir < out[j].RelativeDir
	})
	return out
}

func ensureRootAgentsFile(projectRoot string, m *workspace.Manifest) error {
	path := filepath.Join(projectRoot, GuideFilename(ProviderCodex))
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return os.WriteFile(path, []byte(RootAgentsContent(workspaceName(m))), 0o644)
}

func writeClaudePointer(projectRoot string, force bool) error {
	path := filepath.Join(projectRoot, GuideFilename(ProviderClaudeCode))
	content := ClaudePointerContent()
	current, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return os.WriteFile(path, []byte(content), 0o644)
	}
	if string(current) == content {
		return nil
	}
	managed := strings.Contains(string(current), generatedStart) ||
		strings.Contains(string(current), legacyGeneratedStart) ||
		strings.Contains(string(current), subprojectsStart)
	if !managed && !force {
		return cliErrors.New(cliErrors.AI_GUIDE_EXISTS,
			"CLAUDE.md 已存在且不像 One CLI 生成文件，请手动合并或删除后重试。")
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func writeAgentsDir(projectRoot string, m *workspace.Manifest, projects []workspace.ManifestProject) ([]string, error) {
	generated := []string{}
	if err := writeGeneratedFile(projectRoot, filepath.ToSlash(filepath.Join(agentsRootDir, "conventions.md")), ConventionsContent()); err != nil {
		return nil, err
	}
	generated = append(generated, filepath.ToSlash(filepath.Join(agentsRootDir, "conventions.md")))

	projectFiles, err := writeProjectGuides(projectRoot, projects)
	if err != nil {
		return nil, err
	}
	generated = append(generated, projectFiles...)

	opsFiles, err := writeOpsGuides(projectRoot, m, projects)
	if err != nil {
		return nil, err
	}
	generated = append(generated, opsFiles...)
	return generated, nil
}

func writeProjectGuides(projectRoot string, projects []workspace.ManifestProject) ([]string, error) {
	dir := filepath.Join(projectRoot, filepath.FromSlash(agentsProjectsDir))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	wanted := map[string]bool{}
	generated := make([]string, 0, len(projects))
	for _, p := range projects {
		rel := projectGuideRelPath(p.RelativeDir)
		wanted[filepath.Base(rel)] = true
		if err := writeGeneratedFile(projectRoot, rel, renderProjectGuide(p)); err != nil {
			return nil, err
		}
		generated = append(generated, rel)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".md") || wanted[name] {
			continue
		}
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			return nil, err
		}
	}
	return generated, nil
}

func writeOpsGuides(projectRoot string, m *workspace.Manifest, projects []workspace.ManifestProject) ([]string, error) {
	wanted := map[string]string{
		"dev.md": DevOpsContent(),
	}
	if workspace.EnvBackend(m) != "" {
		wanted["secrets.md"] = SecretsOpsContent()
	}
	containerKinds := projectContainerKinds(m, projects)
	if len(containerKinds) > 0 {
		wanted["container.md"] = ContainerOpsContent(containerKinds)
	}
	deployKinds := projectDeployKinds(projects)
	if len(deployKinds) > 0 {
		wanted["deploy.md"] = DeployOpsContent(deployKinds)
	}

	dir := filepath.Join(projectRoot, filepath.FromSlash(agentsOpsDir))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	order := []string{"dev.md", "secrets.md", "container.md", "deploy.md"}
	generated := []string{}
	for _, name := range order {
		content, ok := wanted[name]
		if !ok {
			continue
		}
		rel := filepath.ToSlash(filepath.Join(agentsOpsDir, name))
		if err := writeGeneratedFile(projectRoot, rel, content); err != nil {
			return nil, err
		}
		generated = append(generated, rel)
	}

	for _, name := range order {
		if _, ok := wanted[name]; ok {
			continue
		}
		path := filepath.Join(dir, name)
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
	}
	return generated, nil
}

func writeGeneratedFile(projectRoot, relPath, content string) error {
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	path := filepath.Join(projectRoot, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func renderAgentsIndex(m *workspace.Manifest, projects []workspace.ManifestProject) string {
	projectRoutes := make([]route, 0, len(projects))
	for _, p := range projects {
		label := p.RelativeDir
		if p.TemplateID != "" {
			label = fmt.Sprintf("%s (%s)", p.RelativeDir, p.TemplateID)
		}
		projectRoutes = append(projectRoutes, route{Label: label, Path: projectGuideRelPath(p.RelativeDir)})
	}
	ops := opsRoutes(m, projects)

	var b strings.Builder
	b.WriteString(generatedStart + "\n")
	b.WriteString("Use this index to choose the smallest relevant detail file.\n\n")
	b.WriteString("- Workspace conventions: [`.one/agents/conventions.md`](.one/agents/conventions.md)\n")
	if len(projectRoutes) > 0 {
		b.WriteString("\n### Projects\n\n")
		for _, r := range projectRoutes {
			b.WriteString("- `" + r.Label + "`: [`" + r.Path + "`](" + r.Path + ")\n")
		}
	}
	if len(ops) > 0 {
		b.WriteString("\n### Operations\n\n")
		for _, r := range ops {
			b.WriteString("- " + r.Label + ": [`" + r.Path + "`](" + r.Path + ")\n")
		}
	}
	b.WriteString(generatedEnd + "\n")
	return b.String()
}

func renderProjectGuide(p workspace.ManifestProject) string {
	content := loadTemplateContent(p.TemplateID)
	if content == "" {
		content = renderFallbackGuide(p.TemplateID)
	}
	name := strings.TrimSpace(p.Name)
	if name == "" {
		name = filepath.Base(filepath.FromSlash(p.RelativeDir))
	}

	var b strings.Builder
	b.WriteString("# " + name + "\n\n")
	b.WriteString("- Project path: `" + p.RelativeDir + "`\n")
	if p.TemplateID != "" {
		b.WriteString("- Template: `" + p.TemplateID + "`\n")
	}
	if p.Toolchain != "" {
		b.WriteString("- Toolchain: `" + p.Toolchain + "`\n")
	}
	if p.PackageManager != "" {
		b.WriteString("- Package manager: `" + p.PackageManager + "`\n")
	}
	b.WriteString("\n## Stack Rules\n\n")
	b.WriteString(strings.TrimSpace(content))
	b.WriteString("\n")
	return b.String()
}

func opsRoutes(m *workspace.Manifest, projects []workspace.ManifestProject) []route {
	out := []route{{
		Label: "Dev workflows",
		Path:  filepath.ToSlash(filepath.Join(agentsOpsDir, "dev.md")),
	}}
	if workspace.EnvBackend(m) != "" {
		out = append(out, route{Label: "Environment variables", Path: filepath.ToSlash(filepath.Join(agentsOpsDir, "secrets.md"))})
	}
	if len(projectContainerKinds(m, projects)) > 0 {
		out = append(out, route{Label: "Container builds", Path: filepath.ToSlash(filepath.Join(agentsOpsDir, "container.md"))})
	}
	if len(projectDeployKinds(projects)) > 0 {
		out = append(out, route{Label: "Deployments", Path: filepath.ToSlash(filepath.Join(agentsOpsDir, "deploy.md"))})
	}
	return out
}

func projectContainerKinds(m *workspace.Manifest, projects []workspace.ManifestProject) []string {
	seen := map[string]bool{}
	for _, p := range projects {
		if p.Domains == nil || p.Domains.Container == nil {
			continue
		}
		kind := workspace.ContainerKindForProject(m, p.Name)
		if strings.TrimSpace(kind) == "" {
			kind = workspace.ContainerBackendDocker
		}
		seen[kind] = true
	}
	return mapKeys(seen)
}

func projectDeployKinds(projects []workspace.ManifestProject) []string {
	seen := map[string]bool{}
	for _, p := range projects {
		if p.Domains == nil || p.Domains.Deploy == nil || strings.TrimSpace(p.Domains.Deploy.Kind) == "" {
			continue
		}
		seen[p.Domains.Deploy.Kind] = true
	}
	return mapKeys(seen)
}

func mapKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func projectGuideRelPath(relativeDir string) string {
	name := flattenRelativeDir(relativeDir)
	return filepath.ToSlash(filepath.Join(agentsProjectsDir, name+".md"))
}

func flattenRelativeDir(relativeDir string) string {
	rel := filepath.ToSlash(strings.TrimSpace(relativeDir))
	rel = strings.Trim(rel, "/")
	if rel == "" || rel == "." {
		return "project"
	}
	rel = strings.NewReplacer("/", "-", "\\", "-").Replace(rel)
	rel = strings.Trim(rel, "-")
	if rel == "" {
		return "project"
	}
	return rel
}

func workspaceName(m *workspace.Manifest) string {
	if m == nil || m.Workspace == nil {
		return ""
	}
	return m.Workspace.Name
}
