package updatecheck

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// withIsolatedCache redirects XDG_CACHE_HOME and HOME to a tmpdir so
// load/save touch test-local files only. Mirrors profile pkg's
// withIsolatedConfig.
func withIsolatedCache(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmp)
	t.Setenv("HOME", tmp)
	return tmp
}

func TestCachePath_HonoursXDG(t *testing.T) {
	tmp := withIsolatedCache(t)
	path, err := cachePath()
	if err != nil {
		t.Fatalf("cachePath: %v", err)
	}
	want := filepath.Join(tmp, "one", "update-check.json")
	if path != want {
		t.Errorf("cachePath: got %q, want %q", path, want)
	}
}

func TestCachePath_FallbackToHomeWhenNoXDG(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("HOME", tmp)
	path, err := cachePath()
	if err != nil {
		t.Fatalf("cachePath: %v", err)
	}
	want := filepath.Join(tmp, ".cache", "one", "update-check.json")
	if path != want {
		t.Errorf("cachePath: got %q, want %q", path, want)
	}
}

func TestLoadCache_MissingFile_ReturnsNilNoError(t *testing.T) {
	withIsolatedCache(t)
	c, err := loadCache()
	if err != nil {
		t.Fatalf("loadCache: %v", err)
	}
	if c != nil {
		t.Errorf("expected nil cache, got %+v", c)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	withIsolatedCache(t)
	now := time.Now().UTC().Truncate(time.Second)
	in := &Cache{
		LastChecked:   now,
		LatestVersion: "v0.9.0",
	}
	if err := saveCache(in); err != nil {
		t.Fatalf("saveCache: %v", err)
	}
	out, err := loadCache()
	if err != nil {
		t.Fatalf("loadCache: %v", err)
	}
	if out == nil {
		t.Fatal("loadCache returned nil after save")
	}
	if out.Schema != cacheSchema {
		t.Errorf("schema: got %q, want %q", out.Schema, cacheSchema)
	}
	if out.LatestVersion != in.LatestVersion {
		t.Errorf("LatestVersion: got %q, want %q", out.LatestVersion, in.LatestVersion)
	}
	if !out.LastChecked.Equal(in.LastChecked) {
		t.Errorf("LastChecked drift: got %v, want %v", out.LastChecked, in.LastChecked)
	}
}

func TestLoadCache_CorruptJSON_ReturnsError(t *testing.T) {
	withIsolatedCache(t)
	path, _ := cachePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	c, err := loadCache()
	if err == nil {
		t.Errorf("expected error, got cache=%+v", c)
	}
}

func TestSaveCache_AtomicRename_NoPartialFile(t *testing.T) {
	withIsolatedCache(t)
	if err := saveCache(&Cache{LastChecked: time.Now(), LatestVersion: "v1.0.0"}); err != nil {
		t.Fatalf("save: %v", err)
	}
	path, _ := cachePath()
	dir := filepath.Dir(path)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	// After save: only the canonical file should remain (no .update-check-*.json
	// temp file leftovers).
	for _, e := range entries {
		if e.Name() != "update-check.json" {
			t.Errorf("unexpected leftover file: %s", e.Name())
		}
	}
}
