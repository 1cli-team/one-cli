package creation

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/template"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/workspace"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/miseconfig"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"gopkg.in/yaml.v3"
)

func TestFlutterTemplates(t *testing.T) {
	registry, err := template.Fetch(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "workspace with spaces")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	for file, body := range map[string]string{
		workspace.ManifestFilename:   "version = 2\n",
		workspace.MiseConfigFilename: "min_version = \"2026.9.7\"\n",
		"pubspec.yaml":               "# Keep my workspace comment\nname: custom_workspace\npublish_to: none\nenvironment:\n  sdk: ^3.13.0\n  flutter: '>=3.47.0'\nworkspace:\n  - packages/extra # Keep existing member\ndependency_overrides:\n  collection: ^1.19.0\n",
	} {
		if err := os.WriteFile(filepath.Join(root, file), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct{ id, name, dir string }{
		{"flutter-app", "sample-app", "apps/sample-app"},
		{"flutter-lib", "shared-ui", "packages/shared-ui"},
	} {
		t.Run(tc.id, func(t *testing.T) {
			var entry *template.Template
			for i := range registry.Templates {
				if registry.Templates[i].ID == tc.id {
					entry = &registry.Templates[i]
				}
			}
			if entry == nil {
				t.Fatalf("missing built-in template %s", tc.id)
			}
			result, err := materializeProject(context.Background(), root, ProjectInput{Template: entry, Name: tc.name, SkipSkills: true})
			if err != nil {
				t.Fatal(err)
			}
			if result.Toolchain != "flutter" || result.PackageManager != "pub" {
				t.Fatalf("unexpected Flutter project metadata: %+v", result)
			}
			var pubspec struct{ Name, Resolution string }
			raw, err := os.ReadFile(filepath.Join(root, tc.dir, "pubspec.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			if err := yaml.Unmarshal(raw, &pubspec); err != nil {
				t.Fatal(err)
			}
			if pubspec.Name != strings.ReplaceAll(tc.name, "-", "_") || pubspec.Resolution != "workspace" {
				t.Fatalf("invalid pub package metadata: %+v", pubspec)
			}
			var mise struct {
				Tools map[string]string
				Tasks map[string]struct {
					Run, Dir    string
					Interactive bool
				}
			}
			raw, err = os.ReadFile(filepath.Join(root, workspace.MiseConfigFilename))
			if err != nil {
				t.Fatal(err)
			}
			if err := toml.Unmarshal(raw, &mise); err != nil {
				t.Fatal(err)
			}
			if mise.Tools["flutter"] == "" || mise.Tools["dart"] != "" || mise.Tools["node"] != "" || mise.Tools["task"] == "" {
				t.Fatalf("Flutter SDK should supply Dart: %+v", mise.Tools)
			}
			operations := []string{"check", "test", "format"}
			if tc.id == "flutter-app" {
				operations = append(operations, "build", "platforms")
			}
			for _, name := range operations {
				task, ok := mise.Tasks[tc.name+":"+name]
				if !ok || task.Run != "task "+name+" --" || task.Dir != tc.dir {
					t.Errorf("missing task %s: %+v", name, task)
				}
			}
			if tc.id == "flutter-app" && !mise.Tasks[tc.name+":dev"].Interactive {
				t.Error("flutter run must retain its interactive terminal for hot reload")
			}
		})
	}
	raw, err := os.ReadFile(filepath.Join(root, "pubspec.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# Keep my workspace comment", "custom_workspace", "collection", "# Keep existing member", "apps/sample-app", "packages/shared-ui"} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("root pubspec lost %q:\n%s", want, raw)
		}
	}
	for _, file := range []string{"package.json", "pnpm-workspace.yaml", "go.work", "pyproject.toml"} {
		if _, err := os.Stat(filepath.Join(root, file)); !os.IsNotExist(err) {
			t.Errorf("Flutter-only workspace created %s: %v", file, err)
		}
	}
}

func TestFlutterFreshWorkspaceAndNameCollision(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, workspace.ManifestFilename), []byte("version = 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	registry, err := template.Fetch(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	var entry *template.Template
	for i := range registry.Templates {
		if registry.Templates[i].ID == "flutter-lib" {
			entry = &registry.Templates[i]
		}
	}
	if entry == nil {
		t.Fatal("missing Flutter library template")
	}
	if _, err := materializeProject(context.Background(), root, ProjectInput{Template: entry, Name: "DataKit", SkipSkills: true}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "pubspec.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	pubspec, err := workspace.ParseFlutterProject(raw)
	if err != nil {
		t.Fatal(err)
	}
	if pubspec.Environment["sdk"] != "^3.6.0" || len(pubspec.Workspace) != 1 || pubspec.Workspace[0] != "packages/DataKit" {
		t.Fatalf("invalid native workspace: %+v", pubspec)
	}
	before, err := os.ReadFile(filepath.Join(root, workspace.ManifestFilename))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := materializeProject(context.Background(), root, ProjectInput{Template: entry, Name: "data-kit", SkipSkills: true}); err == nil {
		t.Fatal("pub package name collision was accepted")
	}
	after, err := os.ReadFile(filepath.Join(root, workspace.ManifestFilename))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("failed creation changed the workspace manifest")
	}
	if _, err := os.Stat(filepath.Join(root, "packages", "data-kit")); !os.IsNotExist(err) {
		t.Fatalf("failed creation left a project directory: %v", err)
	}
	plan, err := miseconfig.Build(root, miseconfig.Options{FlutterVersion: "3.47.8"})
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	plan, err = miseconfig.Build(root, miseconfig.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Changes) != 0 {
		t.Fatalf("mise synchronization changed an existing SDK pin: %+v", plan.Changes)
	}
}

func TestFlutterTemplateLocalization(t *testing.T) {
	defer i18n.Init(i18n.DefaultLocale)
	registry, err := template.Fetch(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	for _, locale := range []string{"en-US", "zh-CN"} {
		if err := i18n.Init(locale); err != nil {
			t.Fatal(err)
		}
		for _, entry := range registry.Templates {
			if entry.Toolchain != template.ToolchainFlutter {
				continue
			}
			for _, suffix := range []string{"name", "description"} {
				key := "template." + entry.ID + "." + suffix
				if i18n.T(key) == key {
					t.Errorf("missing %s translation: %s", locale, key)
				}
			}
		}
	}
}
