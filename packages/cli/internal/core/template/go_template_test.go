package template

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/mod/modfile"
)

func TestGoAPIDeliversChecksumsForRenderedModule(t *testing.T) {
	for _, name := range []string{"api", "billing-service"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			if err := Render("go-api", root, CommonVariables(name, "")); err != nil {
				t.Fatal(err)
			}
			b, err := os.ReadFile(filepath.Join(root, "go.mod"))
			if err != nil {
				t.Fatal(err)
			}
			m, err := modfile.Parse("go.mod", b, nil)
			if err != nil {
				t.Fatal(err)
			}
			if m.Module.Mod.Path != "github.com/example/"+name {
				t.Fatalf("module=%s", m.Module.Mod.Path)
			}
			sums, err := os.ReadFile(filepath.Join(root, "go.sum"))
			if err != nil {
				t.Fatalf("template missing delivered go.sum: %v", err)
			}
			for _, dep := range m.Require {
				if !strings.Contains(string(sums), dep.Mod.Path+" "+dep.Mod.Version+" h1:") {
					t.Errorf("missing checksum for %s@%s", dep.Mod.Path, dep.Mod.Version)
				}
			}
			if err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if !d.IsDir() && strings.HasSuffix(path, ".go") {
					b, err := os.ReadFile(path)
					if err != nil {
						return err
					}
					if strings.Contains(string(b), "{{projectName") {
						t.Errorf("unrendered import in %s", path)
					}
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestGoLibraryDoesNotNeedEmptyChecksumFile(t *testing.T) {
	root := t.TempDir()
	if err := Render("go-lib", root, CommonVariables("shared", "")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.sum")); !os.IsNotExist(err) {
		t.Fatal("standard-library-only module should not need an empty go.sum")
	}
}
