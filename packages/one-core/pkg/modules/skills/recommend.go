package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/mod/modfile"

	"github.com/torchstellar-team/one-cli/packages/one-core/internal/resources/bundled"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/workspace"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/fsutil"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

type RecommendedSkill struct {
	Name        string   `json:"name"`
	Source      string   `json:"source"`
	Recommended bool     `json:"recommended"`
	Selected    bool     `json:"selected"`
	Status      string   `json:"status"`
	Reasons     []string `json:"reasons"`
	Message     string   `json:"message,omitempty"`
}

type RecommendationResult struct {
	Schema   string             `json:"schema"`
	Root     string             `json:"root"`
	DryRun   bool               `json:"dryRun"`
	Skills   []RecommendedSkill `json:"skills"`
	Warnings []string           `json:"warnings,omitempty"`
	Scope    string             `json:"scope"`
}

func (r *RecommendationResult) RenderTTY(w io.Writer) {
	fmt.Fprintln(w, i18n.T("skills.recommend.preview"))
	for _, skill := range r.Skills {
		status := skill.Status
		if r.DryRun && skill.Selected {
			status = "would_install"
		}
		fmt.Fprintf(w, "  %s: %s", skill.Name, i18n.T("skills.recommend.status."+status))
		fmt.Fprintln(w)
		if skill.Message != "" {
			fmt.Fprintf(w, "    %s\n", skill.Message)
		}
	}
	for _, warning := range r.Warnings {
		fmt.Fprintln(w, warning)
	}
}

// InstallGroups passes selected sources and explicit targets to the upstream API.
func (r *RecommendationResult) InstallGroups(names []string, options InstallOptions) ([]InstallGroup, error) {
	known := make(map[string]RecommendedSkill, len(r.Skills))
	for _, skill := range r.Skills {
		known[skill.Name] = skill
	}
	selected := make(map[string]bool, len(names))
	for _, name := range names {
		skill, ok := known[name]
		if !ok {
			return nil, i18n.Errorf("skills.recommend.unknown", name)
		}
		if skill.Status == "blocked" {
			return nil, i18n.Errorf("skills.recommend.unavailable", name, skill.Message)
		}
		selected[name] = true
	}
	var groups []InstallGroup
	indices := map[string]int{}
	for _, skill := range r.Skills {
		if !selected[skill.Name] || skill.Status != "missing" {
			continue
		}
		index, ok := indices[skill.Source]
		if !ok {
			index = len(groups)
			indices[skill.Source] = index
			groups = append(groups, InstallGroup{Source: skill.Source, Agents: slices.Clone(options.Agents)})
		}
		groups[index].Names = append(groups[index].Names, skill.Name)
	}
	return groups, nil
}

// Scope changes only the installation inventory. Recommendations still come
// from the workspace's projects; project source overrides stay project-scoped.
func (r *RecommendationResult) ForScope(scope string, inspection *Inspection) (*RecommendationResult, error) {
	result := *r
	result.Scope = scope
	result.Skills = slices.Clone(r.Skills)
	result.Warnings = slices.Clone(r.Warnings)
	if scope == "global" {
		catalog, err := recommendationCatalog()
		if err != nil {
			return nil, err
		}
		for index := range result.Skills {
			skill := &result.Skills[index]
			for _, selection := range catalog {
				if selection.Names[0] != skill.Name {
					continue
				}
				skill.Source = selection.Source
				if skill.Source == "" {
					skill.Source = oneSource + "/packages/agent-skills#master"
				}
				break
			}
			skill.Status, skill.Message = "missing", ""
		}
	}
	for index := range result.Skills {
		skill := &result.Skills[index]
		if inspection != nil {
			for _, installed := range inspection.Installed {
				if installed.Scope == scope && installed.Name == skill.Name {
					skill.Status = "installed"
					break
				}
			}
		}
		skill.Selected = skill.Recommended && skill.Status == "missing"
	}
	return &result, nil
}

// Recommend only reads local project files. It never prepares a runtime, fetches
// remote catalogs or modifies skills. The catalog is the union of the same
// bundled template defaults used by create/add.
func Recommend(ctx context.Context, root string) (*RecommendationResult, error) {
	manifest, err := workspace.ReadManifest(root)
	if err != nil {
		return nil, err
	}
	if err := fsutil.SafeWritePath(root, filepath.Join(root, "skills-lock.json")); err != nil {
		return nil, err
	}
	lock, err := readLock(root)
	if err != nil {
		return nil, err
	}
	catalog, err := recommendationCatalog()
	if err != nil {
		return nil, err
	}
	result := &RecommendationResult{Schema: "one-cli/recommend-skills/v1", Root: root, DryRun: true, Scope: "project", Skills: []RecommendedSkill{}}
	reasons := map[string][]string{}
	add := func(selections []Selection, reason string, pnpm bool) {
		for _, selection := range selections {
			for _, name := range selection.Names {
				if name == "pnpm" && !pnpm {
					continue
				}
				if !slices.Contains(reasons[name], reason) {
					reasons[name] = append(reasons[name], reason)
				}
			}
		}
	}
	add(Defaults("", ""), i18n.T("skills.recommend.workspace"), false)
	pnpm, err := usesPNPM(root)
	if err != nil {
		result.Warnings = append(result.Warnings, i18n.Tf("skills.recommend.scan_error", root, err))
	}
	if pnpm {
		add([]Selection{{Names: []string{"pnpm"}}}, i18n.Tf("skills.recommend.project", i18n.T("skills.recommend.workspace_name"), "pnpm"), true)
	}
	for _, project := range manifest.Projects {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		detected, err := detectProject(root, project)
		if err != nil {
			result.Warnings = append(result.Warnings, i18n.Tf("skills.recommend.scan_error", project.Name, err))
			continue
		}
		for _, stack := range detected {
			if stack.template == "" && stack.pnpm {
				add([]Selection{{Names: []string{"pnpm"}}}, i18n.Tf("skills.recommend.project", project.Name, "pnpm"), true)
				continue
			}
			selections := Defaults(stack.template, "local:"+stack.template)
			// Workspace guidance has one common reason, not one per project.
			for index := range selections {
				selections[index].Names = slices.DeleteFunc(slices.Clone(selections[index].Names), func(name string) bool {
					return name == "one-cli" || name == "find-skills" || (name == "vite" && !stack.vite)
				})
			}
			add(selections, i18n.Tf("skills.recommend.project", project.Name, stack.label), pnpm || stack.pnpm)
		}
	}
	for _, selection := range catalog {
		name := selection.Names[0]
		skill := RecommendedSkill{Name: name, Source: selection.Source, Status: "missing", Reasons: reasons[name]}
		if skill.Reasons == nil {
			skill.Reasons = []string{}
		}
		if selection.Source == "" {
			skill.Source = oneSource + "/packages/agent-skills#master"
		}
		skill.Recommended = len(skill.Reasons) > 0
		exists, err := installed(root, name)
		if err != nil {
			return nil, err
		}
		entry, locked := lock[name]
		if exists {
			skill.Status = "installed"
			if locked {
				skill.Source = entry.Source
				if entry.Ref != "" {
					skill.Source += "#" + entry.Ref
				}
			}
		} else if locked && entry.SourceType == "github" {
			skill.Source, err = restoreSource(name, entry)
		} else if locked && !sameSource(entry, selection.Source) {
			err = i18n.Errorf("skills.recommend.source_unsupported", name, entry.Source)
		} // Missing legacy local One copies can be installed from the official source.
		if !exists {
			if err == nil {
				err = fsutil.SafeWritePath(root, filepath.Join(root, ".agents", "skills", name))
			}
			if err != nil {
				skill.Status, skill.Message = "blocked", err.Error()
			}
		}
		skill.Selected = skill.Recommended && skill.Status == "missing"
		result.Skills = append(result.Skills, skill)
	}
	slices.SortStableFunc(result.Skills, func(a, b RecommendedSkill) int {
		rank := func(s RecommendedSkill) int {
			if s.Status == "missing" && s.Recommended {
				return 0
			}
			if s.Status == "missing" {
				return 1
			}
			return 2
		}
		if d := rank(a) - rank(b); d != 0 {
			return d
		}
		return strings.Compare(a.Name, b.Name)
	})
	return result, nil
}

func recommendationCatalog() ([]Selection, error) {
	var registry struct{ Templates []struct{ ID, Repo string } }
	if err := json.Unmarshal(bundled.RegistryBytes, &registry); err != nil {
		return nil, err
	}
	byName := map[string]Selection{}
	for _, template := range registry.Templates {
		for _, selection := range Defaults(template.ID, template.Repo) {
			for _, name := range selection.Names {
				byName[name] = Selection{Source: selection.Source, Names: []string{name}}
			}
		}
	}
	catalog := make([]Selection, 0, len(byName))
	for _, selection := range byName {
		catalog = append(catalog, selection)
	}
	slices.SortFunc(catalog, func(a, b Selection) int { return strings.Compare(a.Names[0], b.Names[0]) })
	return catalog, nil
}

// Match Skills CLI's owner/repo/subdirectory#ref input, including slash refs.
func restoreSource(name string, entry lockEntry) (string, error) {
	parts := strings.Split(entry.Source, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" || strings.ContainsAny(entry.Source, ":#?&\\ \t\r\n") || strings.ContainsAny(entry.Ref, "#&\r\n") {
		return "", i18n.Errorf("skills.recommend.source_unsupported", name, entry.Source)
	}
	source := entry.Source
	if entry.SkillPath != "" {
		p := strings.TrimPrefix(entry.SkillPath, "./")
		if !fs.ValidPath(p) || strings.Contains(p, "\\") || path.Base(p) != "SKILL.md" {
			return "", i18n.Errorf("skills.recommend.path_unsupported", entry.SkillPath)
		}
		if dir := path.Dir(p); dir != "." {
			source += "/" + dir
		}
	}
	if entry.Ref != "" {
		source += "#" + entry.Ref
	}
	return source, nil
}

type detectedStack struct {
	template, label string
	vite, pnpm      bool
}

func detectProject(root string, project workspace.ManifestProject) ([]detectedStack, error) {
	read := func(relative string) ([]byte, error) { return readProjectFile(root, relative) }
	dirs, err := workspace.NodeProjectPackageDirs(root, project.RelativeDir, read)
	if err != nil {
		return nil, err
	}
	var result []detectedStack
	add := func(id, label string, vite, pnpm bool) {
		result = append(result, detectedStack{template: id, label: label, vite: vite, pnpm: pnpm})
	}
	for _, dir := range dirs {
		raw, err := read(path.Join(dir, "package.json"))
		if err != nil {
			return nil, err
		}
		if raw == nil {
			continue
		}
		var pkg struct {
			PackageManager                                      string `json:"packageManager"`
			Dependencies, DevDependencies, OptionalDependencies map[string]string
		}
		if err := json.Unmarshal(raw, &pkg); err != nil {
			return nil, err
		}
		has := func(name string) bool {
			return pkg.Dependencies[name] != "" || pkg.DevDependencies[name] != "" || pkg.OptionalDependencies[name] != ""
		}
		vite, pnpm := has("vite"), strings.HasPrefix(pkg.PackageManager, "pnpm@")
		if pnpm {
			add("", "pnpm", false, true)
		}
		switch {
		case has("fumadocs-core") || has("fumadocs-ui") || has("fumadocs-mdx"):
			add("fumadocs-docs", "Fumadocs", vite, pnpm)
		case has("expo"):
			add("expo-mobile", "Expo", vite, pnpm)
		case has("next"):
			add("nextjs-app", "Next.js", vite, pnpm)
		case has("electron"):
			add("electron-app", "Electron", vite, pnpm)
		case (has("react") || has("react-dom")) && !has("react-native"):
			add("react-spa", "React", vite, pnpm)
		}
		if has("@nestjs/core") {
			add("nestjs-api", "NestJS", vite, pnpm)
		}
		if has("tsdown") {
			add("ts-library", "tsdown", vite, pnpm)
		}
	}
	raw, err := read(path.Join(project.RelativeDir, "go.mod"))
	if err != nil {
		return nil, err
	}
	if raw != nil {
		if _, err := modfile.Parse("go.mod", raw, nil); err != nil {
			return nil, err
		}
		add("go-lib", "Go", false, false)
	}
	return result, nil
}

func usesPNPM(root string) (bool, error) {
	raw, err := readProjectFile(root, "pnpm-workspace.yaml")
	if err != nil || raw != nil {
		return raw != nil, err
	}
	raw, err = readProjectFile(root, "package.json")
	if err != nil || raw == nil {
		return false, err
	}
	var pkg struct {
		PackageManager string `json:"packageManager"`
	}
	if err := json.Unmarshal(raw, &pkg); err != nil {
		return false, err
	}
	return strings.HasPrefix(pkg.PackageManager, "pnpm@"), nil
}

func readProjectFile(root, relative string) ([]byte, error) {
	p := filepath.Join(root, filepath.FromSlash(relative))
	if err := fsutil.SafeWritePath(root, p); err != nil {
		return nil, err
	}
	info, err := os.Stat(p)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, i18n.Errorf("skills.untracked_path", p)
	}
	return os.ReadFile(p)
}
