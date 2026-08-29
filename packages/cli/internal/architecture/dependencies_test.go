package architecture_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

const internalImportPrefix = "github.com/torchstellar-team/one-cli/packages/cli/internal/"

// TestDependencyDirection turns the documented dependency rules into an
// executable constraint. Bootstrap is the outer composition root; platform
// and resources are leaves shared by the layers above them.
func TestDependencyDirection(t *testing.T) {
	forbidden := map[string][]string{
		"platform":    {"resources", "core", "ports", "application", "adapters", "modules", "transport", "bootstrap"},
		"resources":   {"platform", "core", "ports", "application", "adapters", "modules", "transport", "bootstrap"},
		"core":        {"ports", "application", "adapters", "modules", "transport", "bootstrap"},
		"ports":       {"resources", "application", "adapters", "modules", "transport", "bootstrap"},
		"application": {"resources", "adapters", "modules", "transport", "bootstrap"},
		"adapters":    {"resources", "application", "modules", "transport", "bootstrap"},
		"modules":     {"transport", "bootstrap"},
		"transport":   {"adapters", "bootstrap"},
	}

	_, currentFile, _, ok := runtimeCaller()
	if !ok {
		t.Fatal("resolve architecture test path")
	}
	internalRoot := filepath.Dir(filepath.Dir(currentFile))

	for layer, blockedLayers := range forbidden {
		layer := layer
		blockedLayers := blockedLayers
		t.Run(layer, func(t *testing.T) {
			root := filepath.Join(internalRoot, layer)
			err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				// Embedded template sources are data, not part of the CLI's Go
				// dependency graph.
				if entry.IsDir() && strings.HasPrefix(entry.Name(), "_") {
					return filepath.SkipDir
				}
				// Unit tests may use a concrete adapter as a fixture. Production
				// packages are the dependency graph this guard protects.
				if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
					return nil
				}
				parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
				if err != nil {
					return err
				}
				for _, imported := range parsed.Imports {
					importPath, err := strconv.Unquote(imported.Path.Value)
					if err != nil || !strings.HasPrefix(importPath, internalImportPrefix) {
						continue
					}
					relativeImport := strings.TrimPrefix(importPath, internalImportPrefix)
					for _, blocked := range blockedLayers {
						if relativeImport == blocked || strings.HasPrefix(relativeImport, blocked+"/") {
							relativeFile, _ := filepath.Rel(internalRoot, path)
							t.Errorf("%s imports outer layer %s", filepath.ToSlash(relativeFile), importPath)
						}
					}
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

// runtimeCaller is a small seam that keeps the path lookup readable and easy
// to replace if this package is ever moved.
var runtimeCaller = func() (uintptr, string, int, bool) {
	return runtime.Caller(0)
}
