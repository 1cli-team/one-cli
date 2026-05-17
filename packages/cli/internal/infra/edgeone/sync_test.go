package edgeone

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
	if err := os.WriteFile(filepath.Join(dir, EdgeOneConfigFilename), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("seed edgeone.json: %v", err)
	}
	if ShouldSync(dir) {
		t.Errorf("ShouldSync with existing edgeone.json = true, want false")
	}
}

func TestSyncWritesProjectNameAndOutputDir(t *testing.T) {
	for _, tpl := range []string{"react-spa", "astro-site", "starlight-docs"} {
		t.Run(tpl, func(t *testing.T) {
			dir := t.TempDir()
			if err := Sync(dir, tpl, "demo-eo"); err != nil {
				t.Fatalf("Sync: %v", err)
			}
			got := readEdgeOneJSON(t, dir)
			want := map[string]any{"projectName": "demo-eo", "outputDir": "dist"}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("edgeone.json = %v, want %v", got, want)
			}
		})
	}
}

func TestSyncWritesNextOutputForNextTemplate(t *testing.T) {
	dir := t.TempDir()
	if err := Sync(dir, "nextjs-app", "demo-eo"); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	got := readEdgeOneJSON(t, dir)
	if got["outputDir"] != ".next" {
		t.Errorf("outputDir = %v, want .next", got["outputDir"])
	}
}

func TestSyncWritesMinimalConfigForUnknownTemplate(t *testing.T) {
	dir := t.TempDir()
	if err := Sync(dir, "some-unknown-template", ""); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	got := readEdgeOneJSON(t, dir)
	if len(got) != 0 {
		t.Fatalf("unknown template + empty projectName should produce empty config, got %v", got)
	}
}

func TestSyncIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	pre := []byte(`{"projectName":"custom"}` + "\n")
	if err := os.WriteFile(filepath.Join(dir, EdgeOneConfigFilename), pre, 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := Sync(dir, "react-spa", "different"); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, EdgeOneConfigFilename))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(raw) != string(pre) {
		t.Fatalf("Sync overwrote existing edgeone.json:\nbefore: %s\nafter:  %s", pre, raw)
	}
}

func readEdgeOneJSON(t *testing.T, dir string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, EdgeOneConfigFilename))
	if err != nil {
		t.Fatalf("read edgeone.json: %v", err)
	}
	var v map[string]any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("unmarshal edgeone.json: %v", err)
	}
	return v
}
