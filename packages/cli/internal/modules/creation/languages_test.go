package creation

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/template"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/fsutil"
	"golang.org/x/mod/modfile"
)

func addLanguageProject(t *testing.T, s *Service, root, id, name string) error {
	t.Helper()
	r, err := template.Fetch(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	for i := range r.Templates {
		if r.Templates[i].ID == id {
			_, err := s.AddProject(context.Background(), root, ProjectInput{Template: &r.Templates[i], Name: name, DeferDeployment: true})
			return err
		}
	}
	t.Fatalf("missing template %s", id)
	return nil
}

func TestFirstProjectInitializesLanguageWorkspaceInEitherOrder(t *testing.T) {
	for _, first := range []string{"go", "node"} {
		t.Run(first, func(t *testing.T) {
			s := newCreationService(t)
			root := filepath.Join(t.TempDir(), "workspace with spaces")
			if _, err := s.CreateWorkspace(context.Background(), WorkspaceInput{TargetDir: root, Name: "demo"}); err != nil {
				t.Fatal(err)
			}
			id, name, secondID, secondName := "go-api", "api", "react-spa", "web"
			if first == "node" {
				id, name, secondID, secondName = secondID, secondName, id, name
			}
			if err := addLanguageProject(t, s, root, id, name); err != nil {
				t.Fatal(err)
			}
			absent := "package.json"
			if first == "node" {
				absent = "go.work"
			}
			if _, err := os.Stat(filepath.Join(root, absent)); !os.IsNotExist(err) {
				t.Fatalf("first %s project created %s", first, absent)
			}
			if first == "go" {
				b, _ := os.ReadFile(filepath.Join(root, "go.work"))
				if !strings.Contains(string(b), "./services/api") {
					t.Fatalf("first module not registered: %s", b)
				}
				for _, path := range []string{".husky", ".changeset", "pnpm-workspace.yaml", "go.work.sum"} {
					if _, err := os.Stat(filepath.Join(root, path)); !os.IsNotExist(err) {
						t.Fatalf("unexpected %s", path)
					}
				}
			}
			if err := addLanguageProject(t, s, root, secondID, secondName); err != nil {
				t.Fatal(err)
			}
			if err := addLanguageProject(t, s, root, "go-lib", "shared"); err != nil {
				t.Fatal(err)
			}
			pkg, _ := os.ReadFile(filepath.Join(root, "apps/web/package.json"))
			if !strings.Contains(string(pkg), `"name": "web"`) {
				t.Fatalf("project kept template package name: %s", pkg)
			}
			b, _ := os.ReadFile(filepath.Join(root, "go.work"))
			f, err := modfile.ParseWork("go.work", b, nil)
			if err != nil || len(f.Use) != 2 {
				t.Fatalf("members: %s %v", b, err)
			}
			m, err := workspace.ReadManifest(root)
			if err != nil {
				t.Fatal(err)
			}
			if len(m.Projects) != 3 {
				t.Fatalf("projects = %v", m.Projects)
			}
			plan := fsutil.NewFilePlan(root)
			if err := planLanguages(plan, m); err != nil {
				t.Fatal(err)
			}
			for path, after := range plan.Overlay() {
				before, _ := os.ReadFile(filepath.Join(root, path))
				if string(before) != string(after) {
					t.Errorf("not idempotent: %s", path)
				}
			}
			rootMise, _ := os.ReadFile(filepath.Join(root, workspace.MiseConfigFilename))
			if strings.Contains(string(rootMise), "go =") || !strings.Contains(string(rootMise), "pnpm =") {
				t.Fatalf("wrong root tools: %s", rootMise)
			}
			hk, err := os.ReadFile(filepath.Join(root, workspace.HooksConfigFilename))
			if err != nil {
				t.Fatal(err)
			}
			for _, step := range []string{`["api:format"]`, `["shared:format"]`, `["web:lint"]`, `["web:format"]`} {
				if !strings.Contains(string(hk), step) {
					t.Errorf("missing hk step %s", step)
				}
			}
			rootPackage, err := os.ReadFile(filepath.Join(root, "package.json"))
			if err != nil {
				t.Fatal(err)
			}
			for _, obsolete := range []string{"husky", "commitlint", "changeset"} {
				if strings.Contains(string(rootPackage), obsolete) {
					t.Errorf("obsolete workspace tooling %s", obsolete)
				}
			}
			for _, obsolete := range []string{".husky", "commitlint.config.js", ".changeset"} {
				if _, err := os.Stat(filepath.Join(root, obsolete)); !os.IsNotExist(err) {
					t.Errorf("obsolete workspace file %s", obsolete)
				}
			}
			goMise, _ := os.ReadFile(filepath.Join(root, "services/api", workspace.MiseConfigFilename))
			if !strings.Contains(string(goMise), "go = '1.27.0'") {
				t.Fatalf("Go tool not configured: %s", goMise)
			}
		})
	}
}

func TestAddConflictLeavesManifestAndRootConfigurationsUnchanged(t *testing.T) {
	for _, conflict := range []struct{ file, content, template string }{
		{"go.work", "invalid Go syntax", "go-api"},
		{"pnpm-workspace.yaml", "packages: [ '!apps/*' ]\n", "react-spa"},
		{workspace.MiseConfigFilename, "# user's modified configuration", "go-api"},
		{workspace.HooksConfigFilename, "// user's modified configuration", "go-api"},
	} {
		t.Run(conflict.file, func(t *testing.T) {
			s := newCreationService(t)
			root := filepath.Join(t.TempDir(), "demo")
			if _, err := s.CreateWorkspace(context.Background(), WorkspaceInput{TargetDir: root, Name: "demo"}); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, conflict.file), []byte(conflict.content), 0o644); err != nil {
				t.Fatal(err)
			}
			manifest, _ := os.ReadFile(filepath.Join(root, workspace.ManifestFilename))
			if err := addLanguageProject(t, s, root, conflict.template, "new-project"); err == nil {
				t.Fatal("expected conflict")
			}
			after, _ := os.ReadFile(filepath.Join(root, workspace.ManifestFilename))
			if string(after) != string(manifest) {
				t.Fatal("failed add changed manifest")
			}
			b, _ := os.ReadFile(filepath.Join(root, conflict.file))
			if string(b) != conflict.content {
				t.Fatal("failed add overwrote user file")
			}
			for _, path := range []string{"services/new-project", "apps/new-project", "package.json"} {
				if _, err := os.Stat(filepath.Join(root, path)); !os.IsNotExist(err) {
					t.Fatalf("failed add left %s", path)
				}
			}
		})
	}
}

func TestExistingNodeManagerAndUserFilesArePreserved(t *testing.T) {
	s := newCreationService(t)
	root := filepath.Join(t.TempDir(), "demo")
	if _, err := s.CreateWorkspace(context.Background(), WorkspaceInput{TargetDir: root, Name: "demo"}); err != nil {
		t.Fatal(err)
	}
	user := `{"name":"custom","packageManager":"npm@11.0.0","scripts":{"custom":"echo keep"},"workspaces":["custom/*"]}`
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(user), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := addLanguageProject(t, s, root, "react-spa", "web"); err != nil {
		t.Fatal(err)
	}
	pkg, _ := os.ReadFile(filepath.Join(root, "package.json"))
	for _, keep := range []string{"npm@11.0.0", "echo keep", "custom/*", "apps/web"} {
		if !strings.Contains(string(pkg), keep) {
			t.Fatalf("lost %s: %s", keep, pkg)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "pnpm-workspace.yaml")); !os.IsNotExist(err) {
		t.Fatal("npm workspace converted to pnpm")
	}
	m, _ := workspace.ReadManifest(root)
	if m.Projects[0].PackageManager != "npm" || workspace.ProjectDev(m, "web") != "npm run dev" {
		t.Fatalf("wrong runtime: %+v", m.Projects[0])
	}
	projectPkg, _ := os.ReadFile(filepath.Join(root, "apps/web/package.json"))
	if strings.Contains(string(projectPkg), "pnpm") || !strings.Contains(string(projectPkg), "npm@11.0.0") {
		t.Fatalf("generated project uses another manager: %s", projectPkg)
	}
}
