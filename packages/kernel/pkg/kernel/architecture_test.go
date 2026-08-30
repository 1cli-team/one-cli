package kernel

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

// TestKernelImportsOnlyStandardLibraryAndItself keeps the reusable runtime
// independent from CLI domains, adapters, and vendor packages.
func TestKernelImportsOnlyStandardLibraryAndItself(t *testing.T) {
	t.Parallel()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve kernel source path")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imported := range parsed.Imports {
			importPath, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				continue
			}
			if strings.HasPrefix(importPath, "github.com/torchstellar-team/one-cli/packages/kernel/") {
				continue
			}
			firstSegment, _, _ := strings.Cut(importPath, "/")
			if strings.Contains(firstSegment, ".") {
				t.Errorf("%s imports non-standard package %s", filepath.Base(path), importPath)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
