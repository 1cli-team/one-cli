package store_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/workspace"
	"github.com/torchstellar-team/one-cli/services/desktop-api/internal/store"
)

func TestWorkspaceRegistrationPreservesManagedSettings(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, store.Config{SQLite: filepath.Join(t.TempDir(), "one.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &workspace.Manifest{Version: 2, Workspace: &workspace.ManifestWorkspace{ID: "demo", Name: "Demo"}, Projects: []workspace.ManifestProject{{Name: "api", RelativeDir: "services/api", Toolchain: "go"}}}
	if _, err := db.Register(ctx, m, "source-1"); err != nil {
		t.Fatal(err)
	}
	current, revision, err := db.Load(ctx, "demo")
	if err != nil {
		t.Fatal(err)
	}
	current.Env = &workspace.EnvironmentConfig{ProjectID: "provider-project", Environments: []string{"dev"}}
	if _, err := db.Save(ctx, "demo", current, revision); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Register(ctx, m, "source-1"); err != nil {
		t.Fatal(err)
	}
	current, revision, err = db.Load(ctx, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if current.Env == nil || current.Env.ProjectID != "provider-project" {
		t.Fatal("registration overwrote managed environment")
	}
	if _, err := db.Save(ctx, "demo", current, "0"); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("stale write: %v", err)
	}
	if _, err := db.Save(ctx, "demo", current, revision); err != nil {
		t.Fatal(err)
	}
}

func TestCredentialsAreEncryptedAndBoundToTheirKey(t *testing.T) {
	ctx := context.Background()
	key := make([]byte, 32)
	key[0] = 1
	db, err := store.Open(ctx, store.Config{SQLite: filepath.Join(t.TempDir(), "one.db"), Key: key})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.PutSecret(ctx, "provider", []byte("sensitive-token")); err != nil {
		t.Fatal(err)
	}
	raw, err := db.Secret(ctx, "provider")
	if err != nil || string(raw) != "sensitive-token" {
		t.Fatalf("read credential: %v", err)
	}
	if err := db.DeleteSecret(ctx, "provider"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Secret(ctx, "provider"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("deleted credential: %v", err)
	}
}
