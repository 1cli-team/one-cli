package store_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/workspace"
	"github.com/torchstellar-team/one-cli/services/desktop-api/internal/store"
)

// Run the same behavior contract on both supported databases. PostgreSQL uses a
// unique schema so tests can run concurrently without touching existing data.
func eachStore(t *testing.T, test func(*testing.T, *store.Store)) {
	t.Helper()
	configs := map[string]store.Config{"sqlite": {SQLite: filepath.Join(t.TempDir(), "one.db"), Key: make([]byte, 32)}}
	if dsn := os.Getenv("ONE_TEST_POSTGRES_URL"); dsn != "" {
		admin, err := sql.Open("pgx", dsn)
		if err != nil {
			t.Fatal(err)
		}
		defer admin.Close()
		schema := fmt.Sprintf("one_test_%d", time.Now().UnixNano())
		if _, err = admin.Exec("CREATE SCHEMA " + schema); err != nil {
			t.Fatal(err)
		}
		defer admin.Exec("DROP SCHEMA " + schema + " CASCADE")
		u, err := url.Parse(dsn)
		if err != nil {
			t.Fatal(err)
		}
		q := u.Query()
		q.Set("search_path", schema)
		u.RawQuery = q.Encode()
		configs["postgres"] = store.Config{Postgres: u.String(), Key: make([]byte, 32)}
	}
	for name, cfg := range configs {
		t.Run(name, func(t *testing.T) {
			db, err := store.Open(context.Background(), cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			test(t, db)
		})
	}
}
func fixture(id string) *workspace.Manifest {
	return &workspace.Manifest{Version: 2, Workspace: &workspace.ManifestWorkspace{ID: id, Name: "Example"}, Projects: []workspace.ManifestProject{{Name: "api", RelativeDir: "services/api", Toolchain: "go"}}}
}
func TestStorageContract(t *testing.T) {
	eachStore(t, func(t *testing.T, db *store.Store) {
		ctx := context.Background()
		m := fixture("example")
		if _, err := db.Register(ctx, m, "engineering-1"); err != nil {
			t.Fatal(err)
		}
		current, revision, err := db.Load(ctx, "example")
		if err != nil {
			t.Fatal(err)
		}
		current.Env = &workspace.EnvironmentConfig{ProjectID: "managed-project", Environments: []string{"dev"}}
		if _, err = db.Save(ctx, "example", current, revision); err != nil {
			t.Fatal(err)
		}
		m.Projects = append(m.Projects, workspace.ManifestProject{Name: "ui", RelativeDir: "apps/ui", Toolchain: "node"})
		if _, err = db.Register(ctx, m, "engineering-2"); err != nil {
			t.Fatal(err)
		}
		current, revision, err = db.Load(ctx, "example")
		if err != nil {
			t.Fatal(err)
		}
		if len(current.Projects) != 2 || current.Env.ProjectID != "managed-project" {
			t.Fatalf("incorrect ownership: %+v", current)
		}
		var wg sync.WaitGroup
		results := make(chan error, 2)
		for range 2 {
			wg.Go(func() { _, err := db.Save(ctx, "example", current, revision); results <- err })
		}
		wg.Wait()
		close(results)
		success, conflicts := 0, 0
		for err := range results {
			if err == nil {
				success++
			} else if errors.Is(err, store.ErrConflict) {
				conflicts++
			} else {
				t.Fatal(err)
			}
		}
		if success != 1 || conflicts != 1 {
			t.Fatalf("concurrent writes: successes=%d conflicts=%d", success, conflicts)
		}
		if err = db.PutSecret(ctx, "provider", []byte("private")); err != nil {
			t.Fatal(err)
		}
		if err = db.AddSession(ctx, "hash", time.Now().Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
		dump, err := db.Export(ctx)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(dump)
		for _, s := range []string{"private", "hash", "server-id", "/home/"} {
			if strings.Contains(string(raw), s) {
				t.Fatalf("export leaked %q", s)
			}
		}
		if err = db.Restore(ctx, dump); !errors.Is(err, store.ErrConflict) {
			t.Fatalf("overwrite import: %v", err)
		}
		if err = db.Forget(ctx, "example"); err != nil {
			t.Fatal(err)
		}
		entries, err := db.List(ctx)
		if err != nil || len(entries) != 0 {
			t.Fatalf("forget did not hide entry: %v", err)
		}
		if _, err = db.Register(ctx, m, "engineering-2"); err != nil {
			t.Fatal(err)
		}
		restored, _, err := db.Load(ctx, "example")
		if err != nil || restored.Env.ProjectID != "managed-project" {
			t.Fatalf("forget lost managed settings: %v", err)
		}
		if err = db.RevokeSession(ctx, "hash"); err != nil || db.Session(ctx, "hash") {
			t.Fatalf("session revocation: %v", err)
		}
	})
}
func TestRestoreValidationIsAtomic(t *testing.T) {
	eachStore(t, func(t *testing.T, db *store.Store) {
		ctx := context.Background()
		dump := store.Transfer{Schema: "one-server/export/v1", Preferences: store.Preferences{Locale: "en-US"}, Workspaces: []*workspace.Manifest{fixture("same"), fixture("same")}}
		if err := db.Restore(ctx, dump); err == nil {
			t.Fatal("duplicate identities accepted")
		}
		entries, err := db.List(ctx)
		if err != nil || len(entries) != 0 {
			t.Fatalf("partial import: %v %+v", err, entries)
		}
	})
}

func TestRestoreIntoEmptyInstance(t *testing.T) {
	eachStore(t, func(t *testing.T, db *store.Store) {
		ctx := context.Background()
		m := fixture("restored")
		m.Env = &workspace.EnvironmentConfig{ProjectID: "remote-project", Environments: []string{"dev", "qa"}}
		dump := store.Transfer{Schema: "one-server/export/v1", Preferences: store.Preferences{Locale: "zh-CN", TaskUI: "stream"}, Workspaces: []*workspace.Manifest{m}}
		if err := db.Restore(ctx, dump); err != nil {
			t.Fatal(err)
		}
		m, _, err := db.Load(ctx, "restored")
		if err != nil || m.Env.ProjectID != "remote-project" {
			t.Fatalf("restore: %+v %v", m, err)
		}
		p, err := db.Preferences(ctx)
		if err != nil || p.Locale != "zh-CN" || p.TaskUI != "stream" {
			t.Fatalf("preferences restore: %+v %v", p, err)
		}
	})
}
