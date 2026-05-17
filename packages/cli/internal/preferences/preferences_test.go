package preferences

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAt_MissingFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "preferences.json")
	got, err := LoadAt(path)
	if err != nil {
		t.Fatalf("LoadAt(missing): %v", err)
	}
	if got.Locale != LocaleAuto {
		t.Errorf("missing file should default to LocaleAuto, got %q", got.Locale)
	}
	if got.Version != SchemaVersion {
		t.Errorf("missing file should default Version to SchemaVersion, got %d", got.Version)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("Load on missing file should NOT create it; stat: %v", err)
	}
}

func TestSaveAt_RoundTrip(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "subdir", "preferences.json")
	p := &Preferences{Locale: LocaleZhCN}
	if err := SaveAt(p, path); err != nil {
		t.Fatalf("SaveAt: %v", err)
	}
	if p.Version != SchemaVersion {
		t.Errorf("Save should stamp Version; got %d", p.Version)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("file mode: want 0600, got %o", got)
	}

	got, err := LoadAt(path)
	if err != nil {
		t.Fatalf("LoadAt after Save: %v", err)
	}
	if got.Locale != LocaleZhCN {
		t.Errorf("locale roundtrip: want zh-CN, got %q", got.Locale)
	}
}

func TestSaveAt_RejectsInvalidLocale(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "preferences.json")
	if err := SaveAt(&Preferences{Locale: "zh_CN"}, path); err == nil {
		t.Fatal("SaveAt should reject zh_CN (underscore, wrong format)")
	}
}

func TestLoadAt_CoercesBadLocale(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "preferences.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"locale":"klingon"}`), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	got, err := LoadAt(path)
	if err != nil {
		t.Fatalf("LoadAt: %v", err)
	}
	if got.Locale != LocaleAuto {
		t.Errorf("bad locale should coerce to auto, got %q", got.Locale)
	}
}

func TestIsValidLocale(t *testing.T) {
	good := []string{LocaleAuto, LocaleZhCN, LocaleEnUS}
	for _, v := range good {
		if !IsValidLocale(v) {
			t.Errorf("IsValidLocale(%q) = false, want true", v)
		}
	}
	bad := []string{"", "zh", "zh_CN", "ZH-CN", "klingon"}
	for _, v := range bad {
		if IsValidLocale(v) {
			t.Errorf("IsValidLocale(%q) = true, want false", v)
		}
	}
}
