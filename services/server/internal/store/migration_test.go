package store_test

import (
	"context"
	"database/sql"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/workspace"
	"github.com/torchstellar-team/one-cli/services/desktop-api/internal/store"
	"path/filepath"
	"testing"
)

func TestVersionOneDatabaseIsMigratedWithoutLosingBindings(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "one.db")
	cfg := store.Config{SQLite: path}
	db, err := store.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	m := fixture("existing")
	m.Env = &workspace.EnvironmentConfig{ProjectID: "preserved-binding", Environments: []string{"dev"}}
	if _, err = db.Register(ctx, m, "source"); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	legacy, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = legacy.Exec("ALTER TABLE workspaces DROP COLUMN archived"); err != nil {
		t.Fatal(err)
	}
	if _, err = legacy.Exec("DELETE FROM schema_migrations WHERE version=2"); err != nil {
		t.Fatal(err)
	}
	legacy.Close()
	db, err = store.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	saved, _, err := db.Load(ctx, "existing")
	if err != nil || saved.Env == nil || saved.Env.ProjectID != "preserved-binding" {
		t.Fatalf("migration lost binding: %+v %v", saved, err)
	}
	entries, err := db.List(ctx)
	if err != nil || len(entries) != 1 || entries[0].ID != "existing" {
		t.Fatalf("migration lost data: %+v %v", entries, err)
	}
}
