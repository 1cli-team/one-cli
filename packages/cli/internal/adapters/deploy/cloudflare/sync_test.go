package cloudflare

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShouldSyncTrueWhenMissing(t *testing.T) {
	if !ShouldSync(t.TempDir()) {
		t.Errorf("ShouldSync on empty dir = false, want true")
	}
}

func TestShouldSyncFalseWhenPresent(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, WranglerConfigFilename), []byte("name = \"x\"\n"), 0o644); err != nil {
		t.Fatalf("seed wrangler.toml: %v", err)
	}
	if ShouldSync(dir) {
		t.Errorf("ShouldSync with existing wrangler.toml = true, want false")
	}
}

// Static-asset templates (CSR / SSG / docs) all get an `[assets]` block
// pointing at the build output dir.
func TestSyncWritesAssetsForStaticTemplates(t *testing.T) {
	for _, tpl := range []string{"react-spa", "astro-site", "starlight-docs"} {
		t.Run(tpl, func(t *testing.T) {
			dir := t.TempDir()
			seedPackageJSON(t, dir, `{"name":"demo","devDependencies":{"astro":"^6.0.0"}}`)
			if err := Sync(dir, tpl, "demo"); err != nil {
				t.Fatalf("Sync: %v", err)
			}
			body := readWranglerToml(t, dir)
			if !strings.Contains(body, `name = "demo"`) {
				t.Errorf("wrangler.toml missing name line: %s", body)
			}
			if !strings.Contains(body, "compatibility_date") {
				t.Errorf("wrangler.toml missing compatibility_date: %s", body)
			}
			if !strings.Contains(body, "[assets]") {
				t.Errorf("static template should emit [assets] block: %s", body)
			}
			if !strings.Contains(body, `directory = "./dist"`) {
				t.Errorf("static template should point assets directory at ./dist: %s", body)
			}
			pkg := readPackageJSON(t, dir)
			devDeps := pkg["devDependencies"].(map[string]any)
			if devDeps["wrangler"] != wranglerDevDependencyVersion {
				t.Fatalf("wrangler devDependency missing: %v", devDeps)
			}
		})
	}
}

func TestSyncDoesNotOverwriteExistingWranglerDependency(t *testing.T) {
	dir := t.TempDir()
	seedPackageJSON(t, dir, `{"name":"demo","devDependencies":{"wrangler":"^4.1.0"}}`)
	if err := Sync(dir, "astro-site", "demo"); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	pkg := readPackageJSON(t, dir)
	devDeps := pkg["devDependencies"].(map[string]any)
	if devDeps["wrangler"] != "^4.1.0" {
		t.Fatalf("existing wrangler version was overwritten: %v", devDeps)
	}
}

// SSR (Next.js) gets a skeleton with a docs comment but no `main` line
// — the user wires it after picking an adapter.
func TestSyncWritesSkeletonForNextjs(t *testing.T) {
	dir := t.TempDir()
	if err := Sync(dir, "nextjs-app", "demo"); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	body := readWranglerToml(t, dir)
	if !strings.Contains(body, `name = "demo"`) {
		t.Errorf("wrangler.toml missing name line: %s", body)
	}
	// The skeleton should NOT have an active `main = ...` line — but
	// it DOES point users at the docs in a comment (which our regex
	// would catch). Look for an uncommented `main =` at the start of
	// any line.
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "main =") || strings.HasPrefix(trimmed, "main=") {
			t.Errorf("Next.js skeleton should NOT preset main (let user pick adapter): %s", body)
			break
		}
	}
	if !strings.Contains(body, "developers.cloudflare.com") {
		t.Errorf("Next.js skeleton should point user at framework guide: %s", body)
	}
}

// Unknown templates still produce a valid minimal toml — wrangler can
// auto-detect a lot once the user adds entry-point fields.
func TestSyncWritesMinimalConfigForUnknownTemplate(t *testing.T) {
	dir := t.TempDir()
	if err := Sync(dir, "some-unknown-template", "demo"); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	body := readWranglerToml(t, dir)
	if !strings.Contains(body, `name = "demo"`) {
		t.Errorf("unknown template missing name: %s", body)
	}
	if !strings.Contains(body, "compatibility_date") {
		t.Errorf("unknown template missing compatibility_date: %s", body)
	}
}

// Empty workerName falls back to a placeholder so wrangler.toml is
// still parseable on first run.
func TestSyncFallsBackToDefaultWorkerName(t *testing.T) {
	dir := t.TempDir()
	if err := Sync(dir, "react-spa", ""); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	body := readWranglerToml(t, dir)
	if !strings.Contains(body, `name = "one-app"`) {
		t.Errorf("default worker name missing: %s", body)
	}
}

// Sync is idempotent — existing wrangler.toml is never overwritten.
func TestSyncIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	pre := []byte(`name = "custom"` + "\n")
	if err := os.WriteFile(filepath.Join(dir, WranglerConfigFilename), pre, 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := Sync(dir, "react-spa", "demo"); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, WranglerConfigFilename))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(raw) != string(pre) {
		t.Fatalf("Sync overwrote existing wrangler.toml:\nbefore: %s\nafter:  %s", pre, raw)
	}
}

func readWranglerToml(t *testing.T, dir string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, WranglerConfigFilename))
	if err != nil {
		t.Fatalf("read wrangler.toml: %v", err)
	}
	return string(raw)
}

func seedPackageJSON(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
}

func readPackageJSON(t *testing.T, dir string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		t.Fatalf("read package.json: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("parse package.json: %v", err)
	}
	return out
}
