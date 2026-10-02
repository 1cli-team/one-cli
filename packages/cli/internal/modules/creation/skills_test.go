package creation

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/template"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	skillsmodule "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/skills"
)

type installFunc func(context.Context, string, []skillsmodule.Selection) []string

func (f installFunc) Install(ctx context.Context, root string, selections []skillsmodule.Selection) []string {
	return f(ctx, root, selections)
}

func TestSkillsRunAfterCreationAndCompositeAndCanBeSkipped(t *testing.T) {
	for _, skip := range []bool{false, true} {
		service := newCreationService(t)
		root := filepath.Join(t.TempDir(), "demo")
		calls := 0
		service.Skills = installFunc(func(_ context.Context, directory string, plan []skillsmodule.Selection) []string {
			calls++
			if directory != root {
				t.Fatalf("skills target=%s want workspace %s", directory, root)
			}
			if calls == 1 {
				var names []string
				for _, selection := range plan {
					names = append(names, selection.Names...)
				}
				if !reflect.DeepEqual(names, []string{"one-cli", "find-skills"}) {
					t.Fatalf("creation must install usage and discovery skills: %v", plan)
				}
			}
			manifest, err := workspace.ReadManifest(root)
			if err != nil {
				t.Fatal("skills ran before creation completed", err)
			}
			if calls == 2 {
				if len(manifest.Projects) != 3 || len(manifest.Groups) != 1 {
					t.Fatal("skills ran before the complete Electron group was published")
				}
				for _, project := range manifest.Projects {
					if _, err := os.Stat(filepath.Join(root, project.RelativeDir, "package.json")); err != nil {
						t.Fatal("skills ran before member files existed", err)
					}
				}
			}
			return []string{"installation unavailable; retry manually"}
		})
		created, err := service.CreateWorkspace(context.Background(), WorkspaceInput{TargetDir: root, Name: "demo", SkipSkills: skip})
		if err != nil {
			t.Fatal(err)
		}
		registry, err := template.Fetch(context.Background(), "")
		if err != nil {
			t.Fatal(err)
		}
		entry, err := registry.Resolve("electron-app")
		if err != nil {
			t.Fatal(err)
		}
		added, err := service.AddProject(context.Background(), root, ProjectInput{Template: entry, Name: "desktop", SkipSkills: skip})
		if err != nil || len(added.Project.Projects) != 3 {
			t.Fatalf("valid project failed because of skills: %+v %v", added, err)
		}
		if skip {
			if calls != 0 || len(created.SkillsWarnings) != 0 {
				t.Fatalf("skip called installer: %d %+v", calls, created)
			}
		} else if calls != 2 || len(created.SkillsWarnings) != 1 || len(added.Project.Warnings) == 0 {
			t.Fatalf("auxiliary failure lost: calls=%d create=%+v add=%+v", calls, created, added)
		}
	}
}
