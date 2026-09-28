package bundled_test

// Bundled-asset invariants. Phase 2 of the test plan: gives `mise run test`
// authority equivalent to `task verify-bundled`'s shell `diff -rq` so a
// developer running `go test ./...` (e.g. via IDE) sees drift too.
//
// Checks:
//   - RegistryBytes parses as JSON with the expected top-level shape
//   - Each embedded FS counts files and matches the canonical on-disk
//     source under the repo root
//   - Spot files known to ship are present (registry.json structure and
//     at least one template)
//
// If you add a new template and `mise run test` suddenly fails here,
// run `mise run sync-bundled` first.

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/templatefiles"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/resources/bundled"
)

// repoRoot resolves the monorepo root regardless of test cwd.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	// packages/cli/internal/resources/bundled/bundled_test.go → five levels up.
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..")
}

func TestRegistryBytes_ParsesAndHasTemplates(t *testing.T) {
	if len(bundled.RegistryBytes) == 0 {
		t.Fatal("RegistryBytes is empty — go:embed of registry.json failed")
	}

	var registry struct {
		Templates []struct {
			ID string `json:"id"`
		} `json:"templates"`
	}
	if err := json.Unmarshal(bundled.RegistryBytes, &registry); err != nil {
		t.Fatalf("parse registry.json: %v", err)
	}
	if len(registry.Templates) == 0 {
		t.Fatal("registry.json has no templates")
	}

	// go-api is referenced by E2E tests; missing it would silently
	// break the smoke pyramid.
	wantTemplate := "go-api"
	found := false
	for _, tpl := range registry.Templates {
		if tpl.ID == wantTemplate {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("registry.json missing template %q", wantTemplate)
	}
}

func TestRegistryBytes_MatchesOnDisk(t *testing.T) {
	disk, err := os.ReadFile(filepath.Join(repoRoot(t), "packages", "templates", "registry.json"))
	if err != nil {
		t.Fatalf("read root registry.json: %v", err)
	}
	if string(disk) != string(bundled.RegistryBytes) {
		t.Error("registry.json drift between root and internal/resources/bundled — run 'mise run sync-bundled'")
	}
}

// WebDistFS is special: its content is a build artefact (Vite bundle), not
// a copy of source files. Vite chunk hashes vary per machine, so we don't
// byte-compare against a fresh build like the template tests do.
// Instead we assert the embed structure looks like a real Vite dist:
// index.html present and at least one hashed JS asset under assets/.
func TestWebDistFS_HasViteDist(t *testing.T) {
	files := mustWalk(t, bundled.WebDistFS, bundled.WebDistRoot)
	if !contains(files, "index.html") {
		t.Errorf("WebDistFS missing index.html — run 'mise run sync-bundled'")
	}
	hasJSAsset := false
	for _, f := range files {
		if strings.HasPrefix(f, "assets/") && strings.HasSuffix(f, ".js") {
			hasJSAsset = true
			break
		}
	}
	if !hasJSAsset {
		t.Errorf("WebDistFS has no assets/*.js — run 'mise run sync-bundled'")
	}
}

func TestTemplatesFS_MatchesOnDisk(t *testing.T) {
	embedded := mustWalk(t, bundled.TemplatesFS, bundled.TemplatesRoot)
	for _, name := range embedded {
		if isAgentAsset(name) {
			t.Errorf("retired agent asset embedded in binary: %s", name)
		}
	}
	diskRoot := filepath.Join(repoRoot(t), "packages", "templates")
	disk := mustWalkOSFiltered(t, diskRoot, func(rel string) bool {
		return rel == "registry.json" || templatefiles.Excluded(rel)
	})
	for i, name := range disk {
		packed := templatefiles.PackPath(name)
		original, err := os.ReadFile(filepath.Join(diskRoot, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		actual, err := fs.ReadFile(bundled.TemplatesFS, bundled.TemplatesRoot+"/"+packed)
		if err != nil || !bytes.Equal(actual, original) {
			t.Errorf("bundled content differs for %s: %v", name, err)
		}
		disk[i] = packed
	}

	assertSameFiles(t, "templates", embedded, disk)

	// At least one known template must ship.
	if !hasPrefix(embedded, "go-api/") {
		t.Error("TemplatesFS missing go-api template")
	}
}

func isAgentAsset(path string) bool {
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		switch part {
		case ".one", ".agents", "AGENTS.md", "CLAUDE.md", "SKILL.md":
			return true
		}
	}
	return false
}

// mustWalk returns the sorted list of file paths inside fsys rooted at
// root, with the root prefix stripped so paths align with mustWalkOS.
func mustWalk(t *testing.T, fsys fs.FS, root string) []string {
	t.Helper()
	var out []string
	err := fs.WalkDir(fsys, root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel := strings.TrimPrefix(path, root+"/")
		out = append(out, rel)
		return nil
	})
	if err != nil {
		t.Fatalf("walk fs %s: %v", root, err)
	}
	return out
}

func mustWalkOSFiltered(t *testing.T, root string, skip func(rel string) bool) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		if d.IsDir() {
			if skip != nil && skip(filepath.ToSlash(rel)) {
				return filepath.SkipDir
			}
			return nil
		}
		// Normalise to forward slashes — embed.FS always uses '/'.
		rel = filepath.ToSlash(rel)
		if skip != nil && skip(rel) {
			return nil
		}
		out = append(out, rel)
		return nil
	})
	if err != nil {
		t.Fatalf("walk disk %s: %v", root, err)
	}
	return out
}

// assertSameFiles compares two file lists as sets and reports drift in a
// way that points at `mise run sync-bundled`.
func assertSameFiles(t *testing.T, label string, embedded, disk []string) {
	t.Helper()
	t.Logf("%s: embedded=%d disk=%d", label, len(embedded), len(disk))
	a := toSet(embedded)
	b := toSet(disk)

	var onlyEmbed, onlyDisk []string
	for f := range a {
		if !b[f] {
			onlyEmbed = append(onlyEmbed, f)
		}
	}
	for f := range b {
		if !a[f] {
			onlyDisk = append(onlyDisk, f)
		}
	}
	if len(onlyEmbed) > 0 || len(onlyDisk) > 0 {
		// Cap the output to avoid drowning the log.
		const cap = 5
		if len(onlyEmbed) > cap {
			onlyEmbed = append(onlyEmbed[:cap], "...")
		}
		if len(onlyDisk) > cap {
			onlyDisk = append(onlyDisk[:cap], "...")
		}
		t.Errorf("%s drift between bundled and root — run 'mise run sync-bundled'\n  only in bundled: %v\n  only on disk:    %v",
			label, onlyEmbed, onlyDisk)
	}
}

func toSet(xs []string) map[string]bool {
	s := make(map[string]bool, len(xs))
	for _, x := range xs {
		s[x] = true
	}
	return s
}

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

func hasPrefix(xs []string, prefix string) bool {
	for _, x := range xs {
		if strings.HasPrefix(x, prefix) {
			return true
		}
	}
	return false
}
