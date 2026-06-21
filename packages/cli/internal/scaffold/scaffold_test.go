package scaffold

// Unit tests for scaffold.Generate. Assert every file lands at its
// expected path with the expected JSON shape; cross-binary parity is
// captured separately by the e2e snapshot suite.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

func TestGenerate_FullLayout(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "demo")

	err := Generate(target, Options{
		ProjectName:    "demo",
		PackageManager: "pnpm",
		Docker:         true,
		K8s:            true,
	})
	if err != nil {
		t.Fatalf("Generate() = %v", err)
	}

	// Files we expect to exist after a full create with infra enabled.
	wantFiles := []string{
		"package.json",
		"one.manifest.json",
		"pnpm-workspace.yaml",
		".gitignore",
		"commitlint.config.js",
		".changeset/config.json",
		".husky/pre-commit",
		".husky/commit-msg",
		"docker-compose.yml",
		"k8s/deployment.yaml",
	}
	for _, rel := range wantFiles {
		path := filepath.Join(target, rel)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("missing file %s: %v", rel, err)
		}
	}

	// husky hooks must be executable so the user's shell will run them.
	for _, rel := range []string{".husky/pre-commit", ".husky/commit-msg"} {
		path := filepath.Join(target, rel)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", rel, err)
		}
		mode := info.Mode().Perm()
		if mode&0o111 == 0 {
			t.Errorf("%s is not executable; mode=%v", rel, mode)
		}
	}

	// package.json field order must match what scaffold.buildPackageJSON
	// stamps. As of manifest v2 there is NO `"one"` key — workspace
	// configuration moved to one.manifest.json.
	pkgRaw, err := os.ReadFile(filepath.Join(target, "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	pkg := string(pkgRaw)
	wantOrder := []string{`"name"`, `"private"`, `"version"`, `"packageManager"`, `"scripts"`, `"devDependencies"`}
	prev := -1
	for _, key := range wantOrder {
		idx := strings.Index(pkg, key)
		if idx < 0 {
			t.Errorf("package.json missing key %s", key)
			continue
		}
		if idx <= prev {
			t.Errorf("package.json key order wrong: %s appears before its predecessor", key)
		}
		prev = idx
	}
	if strings.Contains(pkg, `"one"`) {
		t.Errorf("package.json should not carry an `one` block in manifest v2")
	}

	// JSON files must end with a newline (fs-extra parity).
	for _, rel := range []string{"package.json", ".changeset/config.json", "one.manifest.json"} {
		raw, err := os.ReadFile(filepath.Join(target, rel))
		if err != nil {
			t.Fatal(err)
		}
		if len(raw) == 0 || raw[len(raw)-1] != '\n' {
			t.Errorf("%s missing trailing newline (fs-extra emits one)", rel)
		}
	}

	// Manifest must be parseable + carry the current minimum: version,
	// workspace identity, and an empty projects array. The current schema dropped
	// top-level packageManager and ai blocks (per-project still has
	// packageManager; agent docs are generated from the manifest).
	manifestRaw, err := os.ReadFile(filepath.Join(target, "one.manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(manifestRaw, &m); err != nil {
		t.Fatalf("manifest parse: %v", err)
	}
	if v, _ := m["version"].(float64); v != float64(workspace.ManifestVersion) {
		t.Errorf("manifest version=%v; want %d", m["version"], workspace.ManifestVersion)
	}
	if _, has := m["packageManager"]; has {
		t.Errorf("manifest must not carry top-level packageManager; got %v", m["packageManager"])
	}
	if _, has := m["ai"]; has {
		t.Errorf("manifest must not carry top-level ai block; got %v", m["ai"])
	}

	// workspace block must be set at scaffold time so env init can read
	// the workspace identity without re-prompting the user.
	ws, ok := m["workspace"].(map[string]any)
	if !ok {
		t.Fatalf("manifest has no workspace block; got keys=%v", keys(m))
	}
	if ws["name"] != "demo" {
		t.Errorf("workspace.name = %v; want demo", ws["name"])
	}
	if _, has := ws["roots"]; has {
		t.Errorf("workspace must not carry roots override; got %v", ws["roots"])
	}
	id, _ := ws["id"].(string)
	if !strings.HasPrefix(id, "demo-") || len(id) != len("demo-")+6 {
		t.Errorf("workspace.id = %q; want shape demo-<6-hex>", id)
	}
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestGenerate_NoDocker_NoK8s_OmitsInfraFiles(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "demo")

	err := Generate(target, Options{ProjectName: "demo", PackageManager: "pnpm"})
	if err != nil {
		t.Fatalf("Generate() = %v", err)
	}

	// docker-compose.yml and k8s/ must NOT be present.
	if _, err := os.Stat(filepath.Join(target, "docker-compose.yml")); err == nil {
		t.Errorf("docker-compose.yml should not exist when Docker:false")
	}
	if _, err := os.Stat(filepath.Join(target, "k8s")); err == nil {
		t.Errorf("k8s/ should not exist when K8s:false")
	}
}

func TestIsDirectoryEmpty(t *testing.T) {
	tmp := t.TempDir()
	empty, err := IsDirectoryEmpty(tmp)
	if err != nil || !empty {
		t.Errorf("expected fresh tempdir empty; got empty=%v err=%v", empty, err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "x"), []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	empty, err = IsDirectoryEmpty(tmp)
	if err != nil || empty {
		t.Errorf("expected non-empty after write; got empty=%v err=%v", empty, err)
	}

	missing, err := IsDirectoryEmpty(filepath.Join(tmp, "does-not-exist"))
	if err != nil {
		t.Errorf("missing dir should not error: %v", err)
	}
	if !missing {
		t.Errorf("missing dir should be empty=true")
	}
}
