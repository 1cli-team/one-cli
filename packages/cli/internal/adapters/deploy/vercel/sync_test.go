package vercel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestShouldSyncTrueWhenMissing(t *testing.T) {
	if !ShouldSync(t.TempDir()) {
		t.Errorf("ShouldSync on empty dir = false, want true")
	}
}

func TestShouldSyncFalseWhenPresent(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, VercelConfigFilename), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("seed vercel.json: %v", err)
	}
	if ShouldSync(dir) {
		t.Errorf("ShouldSync with existing vercel.json = true, want false")
	}
}

func TestSyncWritesNextjsForNextTemplate(t *testing.T) {
	dir := t.TempDir()
	if err := Sync(dir, "nextjs-app"); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	got := readVercelJSON(t, dir)
	want := map[string]any{"framework": "nextjs"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("vercel.json = %v, want %v", got, want)
	}
}

func TestSyncWritesViteForReactCsrTemplate(t *testing.T) {
	dir := t.TempDir()
	if err := Sync(dir, "react-spa"); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	got := readVercelJSON(t, dir)
	want := map[string]any{"framework": "vite"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("vercel.json = %v, want %v", got, want)
	}
}

func TestSyncWritesAstroForAstroTemplates(t *testing.T) {
	for _, tpl := range []string{"astro-site", "starlight-docs"} {
		t.Run(tpl, func(t *testing.T) {
			dir := t.TempDir()
			if err := Sync(dir, tpl); err != nil {
				t.Fatalf("Sync: %v", err)
			}
			got := readVercelJSON(t, dir)
			want := map[string]any{"framework": "astro"}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("vercel.json = %v, want %v", got, want)
			}
		})
	}
}

func TestSyncWritesEmptyConfigForUnknownTemplate(t *testing.T) {
	dir := t.TempDir()
	if err := Sync(dir, "some-unknown-template"); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	got := readVercelJSON(t, dir)
	if len(got) != 0 {
		t.Fatalf("unknown template should produce empty config, got %v", got)
	}
}

func TestSyncIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	// Pre-existing config should not be overwritten.
	pre := []byte(`{"framework":"custom"}` + "\n")
	if err := os.WriteFile(filepath.Join(dir, VercelConfigFilename), pre, 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := Sync(dir, "nextjs-app"); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, VercelConfigFilename))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(raw) != string(pre) {
		t.Fatalf("Sync overwrote existing vercel.json:\nbefore: %s\nafter:  %s", pre, raw)
	}
}

func readVercelJSON(t *testing.T, dir string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, VercelConfigFilename))
	if err != nil {
		t.Fatalf("read vercel.json: %v", err)
	}
	var v map[string]any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("unmarshal vercel.json: %v", err)
	}
	return v
}
