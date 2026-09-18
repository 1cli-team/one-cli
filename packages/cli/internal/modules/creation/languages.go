package creation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/modules/gowork"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/fsutil"
	"gopkg.in/yaml.v3"
)

func planLanguages(p *fsutil.FilePlan, m *workspace.Manifest) error {
	var nodeDirs, goDirs []string
	for _, project := range m.Projects {
		rel := filepath.ToSlash(filepath.Clean(project.RelativeDir))
		if err := fsutil.SafeWritePath(p.Root, filepath.Join(p.Root, rel, "one.config")); err != nil {
			return err
		}
		switch project.Toolchain {
		case "go":
			goDirs = append(goDirs, rel)
		case "node":
			nodeDirs = append(nodeDirs, rel)
		}
	}
	if len(goDirs) > 0 {
		b, _, err := gowork.Build(p.Root, goDirs, p.Read)
		if err != nil {
			return err
		}
		if err := p.Set("go.work", b, 0o644); err != nil {
			return err
		}
	}
	if len(nodeDirs) > 0 {
		return planNodeWorkspace(p, m, nodeDirs)
	}
	return nil
}

func nodePackageManager(p *fsutil.FilePlan) (string, error) {
	b, err := p.Read("package.json")
	if err != nil {
		return "", err
	}
	manager := ""
	if b != nil {
		var pkg struct {
			PackageManager string `json:"packageManager"`
		}
		if err := json.Unmarshal(b, &pkg); err != nil {
			return "", err
		}
		manager, _, _ = strings.Cut(pkg.PackageManager, "@")
	}
	for _, lock := range []struct{ file, manager string }{{"pnpm-lock.yaml", "pnpm"}, {"package-lock.json", "npm"}, {"yarn.lock", "yarn"}, {"bun.lock", "bun"}, {"bun.lockb", "bun"}} {
		b, err := p.Read(lock.file)
		if err != nil {
			return "", err
		}
		if b == nil {
			continue
		}
		if manager != "" && manager != lock.manager {
			return "", fmt.Errorf("%s conflicts with package manager %s", lock.file, manager)
		}
		manager = lock.manager
	}
	if manager == "" {
		manager = "pnpm"
	}
	switch manager {
	case "pnpm", "npm", "yarn", "bun":
		return manager, nil
	default:
		return "", fmt.Errorf("unsupported package manager %q", manager)
	}
}

// Bundled Node templates may carry their development package name and manager.
// Set those for the newly generated project before it joins the workspace.
func configureNodePackage(p *fsutil.FilePlan, dir, name, manager string) error {
	raw, err := p.Read(filepath.Join(dir, "package.json"))
	if err != nil {
		return err
	}
	var pkg map[string]json.RawMessage
	if err := json.Unmarshal(raw, &pkg); err != nil {
		return err
	}
	if pkg == nil {
		return fmt.Errorf("project package.json must be an object")
	}
	updates := make(map[string]json.RawMessage)
	updates["name"], _ = marshalJSONValue(name)
	// The root packageManager is authoritative, including its pinned version.
	root, err := p.Read("package.json")
	if err != nil {
		return err
	}
	var rootPkg struct {
		PackageManager string `json:"packageManager"`
	}
	if root != nil {
		if err := json.Unmarshal(root, &rootPkg); err != nil {
			return err
		}
	}
	if rootPkg.PackageManager != "" {
		updates["packageManager"], _ = marshalJSONValue(rootPkg.PackageManager)
	} else {
		updates["packageManager"] = nil
	}
	if manager != "pnpm" {
		var scripts map[string]string
		if raw := pkg["scripts"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &scripts); err != nil {
				return err
			}
		}
		scriptUpdates := make(map[string]json.RawMessage)
		for key, command := range scripts {
			if updated := strings.ReplaceAll(command, "pnpm run ", manager+" run "); updated != command {
				scriptUpdates[key], _ = marshalJSONValue(updated)
			}
		}
		if len(scriptUpdates) > 0 {
			updates["scripts"], err = updateJSONFields(pkg["scripts"], scriptUpdates)
			if err != nil {
				return err
			}
		}
	}
	after, err := updateJSONFields(raw, updates)
	if err != nil {
		return err
	}
	return p.Set(filepath.Join(dir, "package.json"), after, 0o644)
}

func planNodeWorkspace(p *fsutil.FilePlan, m *workspace.Manifest, dirs []string) error {
	manager, err := nodePackageManager(p)
	if err != nil {
		return err
	}
	raw, err := p.Read("package.json")
	if err != nil {
		return err
	}
	newRoot := raw == nil
	if newRoot {
		name := filepath.Base(p.Root)
		if m.Workspace != nil {
			name = m.Workspace.Name
		}
		raw, err = marshalJSONValue(buildPackageJSON(name))
		if err != nil {
			return err
		}
		var formatted bytes.Buffer
		if err := json.Indent(&formatted, raw, "", "  "); err != nil {
			return err
		}
		raw = append(formatted.Bytes(), '\n')
	}
	var pkg map[string]json.RawMessage
	if err := json.Unmarshal(raw, &pkg); err != nil {
		return err
	}
	if pkg == nil {
		return fmt.Errorf("root package.json must be an object")
	}
	updates := make(map[string]json.RawMessage)
	if _, ok := pkg["private"]; !ok {
		updates["private"] = json.RawMessage("true")
	}
	if newRoot && manager != "pnpm" {
		updates["packageManager"] = nil
	}
	if manager == "pnpm" {
		if _, ok := pkg["packageManager"]; !ok {
			updates["packageManager"], _ = marshalJSONValue(packageManagerSpec)
		}
		if err := planPNPMWorkspace(p, dirs); err != nil {
			return err
		}
	} else {
		var patterns []string
		var object map[string]json.RawMessage
		if b := pkg["workspaces"]; len(b) > 0 {
			if err := json.Unmarshal(b, &patterns); err != nil {
				if err := json.Unmarshal(b, &object); err != nil || object == nil {
					return fmt.Errorf("invalid package.json workspaces")
				}
				if err := json.Unmarshal(object["packages"], &patterns); err != nil {
					return fmt.Errorf("invalid package.json workspaces.packages: %w", err)
				}
			}
		}
		updated, err := includeProjects(patterns, dirs)
		if err != nil {
			return err
		}
		if len(updated) != len(patterns) {
			b, _ := marshalJSONValue(updated)
			if object != nil {
				b, err = updateJSONFields(pkg["workspaces"], map[string]json.RawMessage{"packages": b})
				if err != nil {
					return err
				}
			}
			updates["workspaces"] = b
		}
	}
	if len(updates) > 0 {
		raw, err = updateJSONFields(raw, updates)
	}
	if err != nil {
		return err
	}
	return p.Set("package.json", raw, 0o644)
}

func includeProjects(patterns, dirs []string) ([]string, error) {
	out := append([]string(nil), patterns...)
	for _, dir := range dirs {
		covered := false
		for _, pattern := range patterns {
			negative := strings.HasPrefix(pattern, "!")
			glob := strings.TrimPrefix(strings.TrimPrefix(pattern, "!"), "./")
			match, err := path.Match(glob, dir)
			if negative && (err != nil || strings.ContainsAny(glob, "{}()") || strings.Contains(glob, "**")) {
				return nil, fmt.Errorf("cannot safely add %s with workspace exclusion %q; update that pattern first", dir, pattern)
			}
			if match && negative {
				return nil, fmt.Errorf("project %s is excluded by workspace pattern %q", dir, pattern)
			}
			covered = covered || match
		}
		if !covered {
			out = append(out, dir)
		}
	}
	return out, nil
}

func planPNPMWorkspace(p *fsutil.FilePlan, dirs []string) error {
	raw, err := p.Read(WorkspaceFilename)
	if err != nil {
		return err
	}
	newFile := raw == nil
	if newFile {
		raw = []byte(pnpmWorkspaceContent)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return err
	}
	if len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return fmt.Errorf("pnpm-workspace.yaml must contain a mapping")
	}
	root := doc.Content[0]
	var packages *yaml.Node
	for i := 0; i < len(root.Content); i += 2 {
		if root.Content[i].Value == "packages" {
			if packages != nil {
				return fmt.Errorf("duplicate packages key in pnpm-workspace.yaml")
			}
			packages = root.Content[i+1]
		}
	}
	if packages == nil {
		packages = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		root.Content = append(root.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "packages"}, packages)
	}
	var patterns []string
	if packages.Kind != yaml.SequenceNode {
		return fmt.Errorf("pnpm-workspace.yaml packages must be an explicit sequence")
	}
	if err := packages.Decode(&patterns); err != nil {
		return err
	}
	updated, err := includeProjects(patterns, dirs)
	if err != nil {
		return err
	}
	if len(updated) != len(patterns) {
		for _, dir := range updated[len(patterns):] {
			packages.Content = append(packages.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: dir})
		}
		var buf bytes.Buffer
		enc := yaml.NewEncoder(&buf)
		enc.SetIndent(2)
		if err := enc.Encode(&doc); err != nil {
			return err
		}
		raw = buf.Bytes()
	} else if !newFile {
		return nil
	}
	return p.Set(WorkspaceFilename, raw, 0o644)
}
