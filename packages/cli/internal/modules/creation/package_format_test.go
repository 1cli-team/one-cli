package creation

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	internaltoolchain "github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/toolchain"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/template"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/fsutil"
)

func TestConfigureNodePackagePreservesTemplateFormatting(t *testing.T) {
	const source = "{\n\t\"name\": \"template\",\n\t\"scripts\": {\n\t\t\"lint\": \"echo '<ready>' && echo done\",\n\t\t\"check\": \"pnpm run lint && pnpm run format\"\n\t},\n\t\"files\": [\"src\", \"dist\"],\n\t\"custom\": { \"command\": \"pnpm run untouched\" },\n\t\"engines\": { \"node\": \">=24\" },\n\t\"packageManager\": \"pnpm@12.3.4\"\n}\n"
	for _, manager := range []string{"pnpm"} {
		t.Run(manager, func(t *testing.T) {
			p := fsutil.NewFilePlan(t.TempDir())
			root := `{"packageManager":"` + manager + `@1.2.3"}`
			if err := p.Set("package.json", []byte(root), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := p.Set("apps/web/package.json", []byte(source), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := configureNodePackage(p, "apps/web", "web", manager); err != nil {
				t.Fatal(err)
			}
			want := strings.Replace(source, `"name": "template"`, `"name": "web"`, 1)
			want = strings.Replace(want, "pnpm@12.3.4", manager+"@1.2.3", 1)
			want = strings.Replace(want, "pnpm run lint && pnpm run format", manager+" run lint && "+manager+" run format", 1)
			got, err := p.Read("apps/web/package.json")
			if err != nil || string(got) != want {
				t.Fatalf("generated package changed unrelated formatting: %v\nwant:\n%s\ngot:\n%s", err, want, got)
			}
			if err := configureNodePackage(p, "apps/web", "web", manager); err != nil {
				t.Fatal(err)
			}
			again, _ := p.Read("apps/web/package.json")
			if string(again) != string(got) {
				t.Fatal("configuration is not idempotent")
			}
		})
	}
}

// Use the real formatter against projects created through the public service,
// including package rewrites and generated files, not just template sources.
func TestGeneratedNodeProjectsPassFormatting(t *testing.T) {
	formatter := os.Getenv("ONE_TEST_OXFMT_BINARY")
	if formatter == "" {
		t.Skip("set ONE_TEST_OXFMT_BINARY to run generated-project formatting checks")
	}
	internaltoolchain.RegisterBundled()
	registry, err := template.Fetch(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range registry.Templates {
		if entry.Toolchain != "node" {
			continue
		}
		t.Run(entry.ID, func(t *testing.T) {
			s := newCreationService(t)
			root := filepath.Join(t.TempDir(), "format-check")
			if _, err := s.CreateWorkspace(context.Background(), WorkspaceInput{TargetDir: root, Name: "format-check"}); err != nil {
				t.Fatal(err)
			}
			result, err := s.AddProject(context.Background(), root, ProjectInput{Template: &entry, Name: "sample"})
			if err != nil {
				t.Fatal(err)
			}
			projects := result.Project.Projects
			if len(projects) == 0 {
				projects = []ProjectResult{result.Project}
			}
			for _, project := range projects {
				cmd := exec.Command(formatter, "--check", ".")
				cmd.Dir = project.TargetPath
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("generated %s formatting: %v\n%s", project.Name, err, out)
				}
				b, err := os.ReadFile(filepath.Join(project.TargetPath, "package.json"))
				var pkg struct{ Name string }
				if err != nil || json.Unmarshal(b, &pkg) != nil || pkg.Name != project.Name {
					t.Fatalf("generated package name: %s (%v)", b, err)
				}
			}
			cmd := exec.Command(formatter, "--check", "package.json")
			cmd.Dir = root
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("workspace package formatting: %v\n%s", err, out)
			}

		})
	}
}
