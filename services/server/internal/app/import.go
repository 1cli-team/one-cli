package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	oneclient "github.com/torchstellar-team/one-cli/packages/one-client"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/preferences"
	"github.com/zalando/go-keyring"
	"os"
	"path/filepath"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/workspace"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/fsutil"
	"github.com/torchstellar-team/one-cli/services/desktop-api/internal/store"
)

// Import records are committed only after successful, repeatable imports.
// Original files are preserved and copied to private, content-addressed backups.
func importLegacy(ctx context.Context, db *store.Store, root string) error {
	for _, name := range []string{"preferences.json", "workspaces.json", "global-env.json"} {
		file := filepath.Join(root, "config", name)
		data, err := os.ReadFile(file)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		digest := hex.EncodeToString(sum[:])
		imported, e := db.Imported(ctx, name)
		if e != nil {
			return e
		}
		if imported {
			continue
		}
		backup := filepath.Join(root, "data", "import-backups", digest+"-"+name)
		if err = os.MkdirAll(filepath.Dir(backup), 0700); err != nil {
			return err
		}
		if err = fsutil.WriteAtomic(backup, data, 0600); err != nil {
			return err
		}
		switch name {
		case "preferences.json":
			var p struct {
				Version int    `json:"version"`
				Locale  string `json:"locale"`
				TaskUI  string `json:"taskUI"`
			}
			if err = json.Unmarshal(data, &p); err != nil {
				return err
			}
			if p.Version > 1 {
				return errors.New("preferences schema is newer than this server")
			}
			if !preferences.IsValidLocale(p.Locale) {
				p.Locale = "auto"
			}
			if p.TaskUI != "stream" && p.TaskUI != "tui" {
				p.TaskUI = ""
			}
			if err = db.SavePreferences(ctx, store.Preferences{Locale: p.Locale, TaskUI: p.TaskUI}); err != nil {
				return err
			}
		case "workspaces.json":
			var registry workspace.Registry
			if err = json.Unmarshal(data, &registry); err != nil {
				return err
			}
			if registry.Version != 1 {
				return errors.New("workspace registry schema is newer than this server")
			}
			for _, entry := range registry.Workspaces {
				if !workspace.HasManifest(entry.Root) {
					continue
				}
				m, revision, e := workspace.ReadManifestSnapshot(entry.Root)
				if e != nil {
					return e
				}
				if m.Workspace == nil {
					continue
				}
				if _, e = db.Register(ctx, m, revision); e != nil {
					return e
				}
				client := oneclient.New()
				client.Root = root
				if e = client.UpdateConnections(ctx, func(p *oneclient.Connections) error {
					if _, ok := p.Bindings[entry.Root]; !ok {
						p.Bindings[entry.Root] = oneclient.Binding{Server: "local"}
					}
					return nil
				}); e != nil {
					return e
				}
			}
		case "global-env.json":
			var value map[string]any
			if err = json.Unmarshal(data, &value); err != nil {
				return err
			}
			if err = db.PutSetting(ctx, "global-env", data); err != nil {
				return err
			}
		}
		if err = db.CompleteImport(ctx, name, digest); err != nil {
			return err
		}
	}
	// The legacy keyring account belongs to the default per-user installation.
	// Isolated ONE_HOME instances must never import another installation's login.
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	if filepath.Clean(root) == filepath.Join(home, ".one") {
		if _, err = db.Setting(ctx, "import:keyring"); errors.Is(err, store.ErrNotFound) {
			raw, e := keyring.Get("one-cli.infisical", "session")
			if e == nil {
				var value map[string]any
				if e = json.Unmarshal([]byte(raw), &value); e != nil {
					return e
				}
				if e = db.PutSecret(ctx, "infisical-session", []byte(raw)); e != nil {
					return e
				}
				if e = db.PutSetting(ctx, "import:keyring", []byte("encrypted")); e != nil {
					return e
				}
			} else if errors.Is(e, keyring.ErrNotFound) {
				if e = db.PutSetting(ctx, "import:keyring", []byte("absent")); e != nil {
					return e
				}
			}
		}
	}
	return nil
}
