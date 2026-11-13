package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/template"
)

func writeWorkspaceFile(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRecommendFromRenderedTemplates(t *testing.T) {
	for _, id := range []string{"go-api", "go-lib", "nestjs-api", "nextjs-app", "nextjs-site", "react-spa", "fumadocs-docs", "expo-mobile", "ts-library", "electron-app", "bun-api", "deno-api", "empty-app"} {
		t.Run(id, func(t *testing.T) {
			root := t.TempDir()
			// Bun/Deno packages inherit pnpm from the workspace scaffold.
			if id == "bun-api" || id == "deno-api" {
				writeWorkspaceFile(t, root, "pnpm-workspace.yaml", "packages: []\n")
			}
			vars := template.CommonVariables("demo", "pnpm")
			projects, err := template.PrepareProjects(id, vars)
			if err != nil {
				t.Fatal(err)
			}
			manifest := "version = 2\n"
			if len(projects) == 0 {
				if err := template.Render(id, filepath.Join(root, "apps/demo"), vars); err != nil {
					t.Fatal(err)
				}
				manifest += "[projects.demo]\npath = 'apps/demo'\ntoolchain = 'none'\n"
			} else {
				for _, project := range projects {
					dir := "parts/" + project.Name
					for name, content := range project.Files {
						writeWorkspaceFile(t, root, dir+"/"+name, string(content))
					}
					manifest += fmt.Sprintf("[projects.%s]\npath = '%s'\ntoolchain = 'node'\n", project.Name, dir)
				}
				writeWorkspaceFile(t, root, "pnpm-workspace.yaml", "packages: []\n")
			}
			writeWorkspaceFile(t, root, "one.manifest.toml", manifest)
			result, err := Recommend(context.Background(), root)
			if err != nil || len(result.Warnings) > 0 {
				t.Fatalf("scan = %+v, %v", result, err)
			}
			for _, selection := range Defaults(id, "local:"+id) {
				for _, name := range selection.Names {
					if !recommendation(t, result, name).Selected {
						t.Errorf("template %s did not recommend %s", id, name)
					}
				}
			}
		})
	}
}

func TestRecommendationRejectsUnsafePathsAndPreservesUnknownSources(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, root, "one.manifest.toml", "version = 2\n[projects.app]\npath = 'apps/app'\ntoolchain = 'node'\n")
	outside := t.TempDir()
	writeWorkspaceFile(t, outside, "package.json", `{"dependencies":{"next":"16"}}`)
	if err := os.MkdirAll(filepath.Join(root, "apps"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "apps/app")); err != nil {
		t.Skip(err)
	}
	writeTestLock(t, root, "one-go", Files{}, "./custom-team-skills", "local", "")
	result, err := Recommend(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Warnings) != 1 || recommendation(t, result, "one-web").Recommended {
		t.Fatal("followed external project symlink")
	}
	if got := recommendation(t, result, "one-go"); got.Status != "blocked" || got.Selected {
		t.Fatalf("unknown source = %+v", got)
	}
	if err := os.Symlink(outside, filepath.Join(root, ".agents")); err != nil {
		t.Fatal(err)
	}
	result, err = Recommend(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if recommendation(t, result, "one-cli").Status != "blocked" {
		t.Fatal("unsafe installation path accepted")
	}
}

func recommendation(t *testing.T, result *RecommendationResult, name string) RecommendedSkill {
	t.Helper()
	for _, skill := range result.Skills {
		if skill.Name == name {
			return skill
		}
	}
	t.Fatalf("skill %s missing from catalog", name)
	return RecommendedSkill{}
}

func TestRecommendMixedWorkspace(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, root, "one.manifest.toml", `version = 2
[projects.web]
path = "apps/web"
toolchain = "node"
[projects.docs]
path = "apps/docs"
toolchain = "node"
[projects.api]
path = "services/api"
toolchain = "go"
[projects.empty]
path = "apps/empty"
toolchain = "none"
`)
	writeWorkspaceFile(t, root, "apps/web/package.json", `{"dependencies":{"next":"16","react":"19"}}`)
	writeWorkspaceFile(t, root, "apps/docs/package.json", `{"dependencies":{"next":"16","fumadocs-core":"16"}}`)
	writeWorkspaceFile(t, root, "services/api/go.mod", "module example.com/api\n\ngo 1.26.0\n")
	// Root tooling must not be mistaken for dependencies of the empty project.
	writeWorkspaceFile(t, root, "package.json", `{"devDependencies":{"expo":"55"}}`)
	putFiles(t, root, "one-cli", Files{"SKILL.md": []byte("team-edited instructions")})
	result, err := Recommend(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, skill := range result.Skills {
		if seen[skill.Name] {
			t.Fatalf("duplicate candidate: %s", skill.Name)
		}
		seen[skill.Name] = true
	}
	for _, name := range []string{"one-web", "one-fumadocs", "one-go", "golang-testing", "vercel-react-best-practices"} {
		skill := recommendation(t, result, name)
		if !skill.Recommended || !skill.Selected || skill.Status != "missing" {
			t.Errorf("missing default selection: %+v", skill)
		}
	}
	if skill := recommendation(t, result, "one-cli"); !skill.Recommended || skill.Selected || skill.Status != "installed" {
		t.Errorf("existing copy should be preserved: %+v", skill)
	}
	if skill := recommendation(t, result, "one-web"); len(skill.Reasons) != 2 {
		t.Errorf("want both contributing projects: %+v", skill)
	}
	if skill := recommendation(t, result, "one-expo"); skill.Recommended || skill.Selected {
		t.Errorf("root tooling leaked into project recommendations: %+v", skill)
	}
	if _, err := os.Stat(filepath.Join(root, "skills-lock.json")); !os.IsNotExist(err) {
		t.Fatalf("preview wrote a lock: %v", err)
	}
}

func TestRecommendDetectsProjectMembersAndKeepsUnknownStacksGeneric(t *testing.T) {
	for _, test := range []struct {
		name, pkg    string
		want, absent []string
	}{
		{"expo", `{"dependencies":{"expo":"55","react":"19","react-native":"0.84"}}`, []string{"one-expo", "expo-router"}, []string{"one-web", "shadcn"}},
		{"nestjs", `{"dependencies":{"@nestjs/core":"11"}}`, []string{"one-nestjs", "nestjs-best-practices"}, []string{"one-web"}},
		{"react vite", `{"dependencies":{"react":"19"},"devDependencies":{"vite":"8"}}`, []string{"one-web", "vite"}, []string{"one-expo"}},
		{"library", `{"devDependencies":{"tsdown":"0.21"}}`, []string{"tsdown"}, []string{"one-web", "one-nestjs"}},
		{"unknown node", `{}`, []string{"one-cli", "find-skills"}, []string{"one-web", "one-nestjs", "one-expo"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writeWorkspaceFile(t, root, "one.manifest.toml", "version = 2\n[projects.app]\npath = 'apps/app'\ntoolchain = 'none'\n")
			writeWorkspaceFile(t, root, "apps/app/package.json", `{"workspaces":["packages/*"]}`)
			writeWorkspaceFile(t, root, "apps/app/packages/client/package.json", test.pkg)
			result, err := Recommend(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			for _, name := range test.want {
				if !recommendation(t, result, name).Selected {
					t.Errorf("%s not recommended", name)
				}
			}
			for _, name := range test.absent {
				if recommendation(t, result, name).Selected {
					t.Errorf("%s incorrectly recommended", name)
				}
			}
		})
	}
}

func TestRecommendedCommandsGroupSelectedMissingSkills(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, root, "one.manifest.toml", "version = 2\n")
	putFiles(t, root, "one-cli", Files{"SKILL.md": []byte("team instructions")})
	result, err := Recommend(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	commands, err := result.InstallGroups([]string{"one-cli", "one-go", "one-web", "golang-testing", "one-go"}, InstallOptions{Agents: []string{"codex"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(commands) != 2 {
		t.Fatalf("groups = %+v", commands)
	}
	want := map[string][]string{oneSource + "/packages/agent-skills#master": {"one-go", "one-web"}, goSource: {"golang-testing"}}
	for _, command := range commands {
		if expected := want[command.Source]; !slices.Equal(command.Names, expected) || !slices.Equal(command.Agents, []string{"codex"}) {
			t.Fatalf("group = %+v, want %q", command, expected)
		}
	}
	if raw, _ := os.ReadFile(filepath.Join(root, ".agents/skills/one-cli/SKILL.md")); string(raw) != "team instructions" {
		t.Fatal("planning modified installed skill")
	}
	if _, err := os.Stat(filepath.Join(root, ".agents/skills/one-go")); !os.IsNotExist(err) {
		t.Fatal("planner installed a skill itself")
	}
	if _, err := os.Stat(filepath.Join(root, "skills-lock.json")); !os.IsNotExist(err) {
		t.Fatal("planner wrote a lock")
	}
}

func TestRecommendedCommandsPreserveRecordedSource(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, root, "one.manifest.toml", "version = 2\n")
	raw := writeTestLock(t, root, "one-go", Files{}, oneSource, "github", "feature/old-guidance")
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	document["skills"].(map[string]any)["one-go"].(map[string]any)["skillPath"] = officialSkillPath("one-go")
	raw, _ = json.Marshal(document)
	writeWorkspaceFile(t, root, "skills-lock.json", string(raw))
	result, err := Recommend(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	commands, err := result.InstallGroups([]string{"one-go"}, InstallOptions{})
	want := "1cli-team/one-cli/packages/agent-skills/skills/one-go#feature/old-guidance"
	if err != nil || len(commands) != 1 || commands[0].Source != want {
		t.Fatalf("groups = %+v, %v", commands, err)
	}
	if after, _ := os.ReadFile(filepath.Join(root, "skills-lock.json")); string(after) != string(raw) {
		t.Fatal("planning changed the lock")
	}
}

func TestRecommendedCommandsEmptySelectionAndNewPlacements(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, root, "one.manifest.toml", "version = 2\n")
	result, err := Recommend(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if commands, err := result.InstallGroups(nil, InstallOptions{}); err != nil || len(commands) != 0 {
		t.Fatalf("empty selection = %+v, %v", commands, err)
	}
	if _, err := result.InstallGroups([]string{"unknown-skill"}, InstallOptions{}); err == nil {
		t.Fatal("unknown skill accepted")
	}
	putFiles(t, root, "one-go", Files{"SKILL.md": []byte("installed while selecting")})
	result, err = Recommend(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if commands, err := result.InstallGroups([]string{"one-go"}, InstallOptions{}); err != nil || len(commands) != 0 {
		t.Fatalf("new placement was not skipped: %+v, %v", commands, err)
	}
}
