package app

import (
	"context"
	"encoding/json"
	"github.com/torchstellar-team/one-cli/services/desktop-api/internal/store"
	"os"
	"path/filepath"
	"testing"
)

func TestLegacyImportIsBackedUpAndDoesNotOverwriteLaterPreferences(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()
	config := filepath.Join(root, "config")
	if err := os.MkdirAll(config, 0700); err != nil {
		t.Fatal(err)
	}
	legacy := []byte(`{"version":1,"locale":"zh-CN","taskUI":"stream"}`)
	path := filepath.Join(config, "preferences.json")
	if err := os.WriteFile(path, legacy, 0600); err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(ctx, store.Config{SQLite: filepath.Join(root, "data", "one.db"), Key: make([]byte, 32)})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = importLegacy(ctx, db, root); err != nil {
		t.Fatal(err)
	}
	preferences, err := db.Preferences(ctx)
	if err != nil || preferences.Locale != "zh-CN" {
		t.Fatalf("import: %+v %v", preferences, err)
	}
	files, err := os.ReadDir(filepath.Join(root, "data", "import-backups"))
	if err != nil || len(files) != 1 {
		t.Fatalf("backup: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "data", "import-backups", files[0].Name()))
	if err != nil || string(raw) != string(legacy) {
		t.Fatal("backup bytes changed")
	}
	if err = db.SavePreferences(ctx, store.Preferences{Locale: "en-US"}); err != nil {
		t.Fatal(err)
	}
	if err = importLegacy(ctx, db, root); err != nil {
		t.Fatal(err)
	}
	preferences, err = db.Preferences(ctx)
	if err != nil || preferences.Locale != "en-US" {
		t.Fatal("repeat import overwrote management data")
	}
	raw, err = os.ReadFile(path)
	if err != nil || !json.Valid(raw) || string(raw) != string(legacy) {
		t.Fatal("legacy source was modified")
	}
}
