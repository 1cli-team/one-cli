package creation

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/template"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/workspace"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

// Exercise the same transaction used by one add, including manifest validation,
// bundled rendering, task discovery and mise generation. No global state is written.
func TestSwiftTemplates(t *testing.T) {
	registry, err := template.Fetch(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"swift-macos-app", "swift-menubar-app", "swift-ios-app"} {
		t.Run(id, func(t *testing.T) {
			var entry *template.Template
			for i := range registry.Templates {
				if registry.Templates[i].ID == id {
					entry = &registry.Templates[i]
				}
			}
			if entry == nil {
				t.Fatalf("missing built-in template %s", id)
			}
			name := "sample-app"
			if id == "swift-menubar-app" {
				name = "Sample_App"
			}
			root := filepath.Join(t.TempDir(), "workspace with spaces")
			if err := os.MkdirAll(root, 0o755); err != nil {
				t.Fatal(err)
			}
			for name, content := range map[string]string{
				workspace.ManifestFilename:   "version = 2\n",
				workspace.MiseConfigFilename: "min_version = \"2026.9.7\"\n",
			} {
				if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			result, err := materializeProject(context.Background(), root, ProjectInput{
				Template: entry, Name: name, SkipSkills: true,
			})
			if err != nil {
				t.Fatal(err)
			}
			if result.Toolchain != "swift" || result.PackageManager != "" {
				t.Fatalf("unexpected Swift project metadata: %+v", result)
			}
			manifest, err := workspace.ReadManifest(root)
			if err != nil {
				t.Fatal(err)
			}
			if len(manifest.Projects) != 1 || manifest.Projects[0].Toolchain != "swift" {
				t.Fatalf("unexpected manifest: %+v", manifest)
			}
			for _, file := range []string{"go.work", "package.json", "pnpm-workspace.yaml"} {
				if _, err := os.Stat(filepath.Join(root, file)); !os.IsNotExist(err) {
					t.Fatalf("Swift-only workspace should not generate %s: %v", file, err)
				}
			}
			var mise struct {
				Tools map[string]string
				Tasks map[string]struct{ Run, Dir string }
			}
			raw, err := os.ReadFile(filepath.Join(root, workspace.MiseConfigFilename))
			if err != nil {
				t.Fatal(err)
			}
			if err := toml.Unmarshal(raw, &mise); err != nil {
				t.Fatal(err)
			}
			if mise.Tools["task"] == "" || mise.Tools["swift"] != "" || mise.Tools["node"] != "" || mise.Tools["go"] != "" {
				t.Fatalf("Swift should use Xcode and install only Task: %+v", mise.Tools)
			}
			for _, task := range []string{"dev", "build", "check", "pack", "release"} {
				op, ok := mise.Tasks[name+":"+task]
				if !ok || op.Run != "task "+task+" --" || op.Dir != "apps/"+name {
					t.Errorf("missing native task %s: %+v", task, op)
				}
			}
			err = filepath.WalkDir(result.TargetPath, func(path string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return err
				}
				raw, err := os.ReadFile(path)
				if err == nil && strings.Contains(string(raw), "one-template-") {
					t.Errorf("unexpanded placeholder in %s", path)
				}
				return err
			})
			if err != nil {
				t.Fatal(err)
			}
			plist, err := os.ReadFile(filepath.Join(result.TargetPath, "Info.plist"))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(plist), "LSUIElement") != (id == "swift-menubar-app") {
				t.Error("only the menu-bar app should hide its Dock icon")
			}
			// Opt-in native smoke test: run on a Mac with Xcode, without UI,
			// personal signing credentials, notarization or network access.
			if id == "swift-ios-app" && runtime.GOOS == "darwin" && os.Getenv("ONE_TEST_IOS_TEMPLATES") == "1" {
				checkNativeIOSTemplate(t, result.TargetPath)
			}
			if id != "swift-ios-app" && runtime.GOOS == "darwin" && os.Getenv("ONE_TEST_MACOS_TEMPLATES") == "1" {
				checkNativeSwiftTemplate(t, result.TargetPath)
			}
		})
	}
}

func TestSwiftTemplateLocalization(t *testing.T) {
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
			if entry.Toolchain != template.ToolchainSwift {
				continue
			}
			nameKey, descKey := "template."+entry.ID+".name", "template."+entry.ID+".description"
			if i18n.T(nameKey) == nameKey || i18n.T(descKey) == descKey ||
				entry.DisplayName() != i18n.T(nameKey) || entry.DisplayDescription() != i18n.T(descKey) {
				t.Errorf("missing %s translation for %s", locale, entry.ID)
			}
		}
	}
}
