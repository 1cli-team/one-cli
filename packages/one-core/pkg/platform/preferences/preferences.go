// Package preferences provides the legacy preference reader and a local startup
// cache. Authoritative preferences are stored by Server; new callers use the
// management client. The cache lets local commands and help start offline.
package preferences

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/fsutil"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/userdirs"
)

// SchemaVersion is the current preferences.json shape version. Bumped
// only on incompatible field renames; new optional fields don't
// require a bump.
const SchemaVersion = 1

// Locale values. Stored verbatim in the file; consumers compare
// against these constants instead of inlining string literals.
const (
	LocaleAuto = "auto"  // follow machine locale (LANG / LC_ALL / LC_MESSAGES)
	LocaleZhCN = "zh-CN" // Simplified Chinese, force
	LocaleEnUS = "en-US" // US English, force
	defaultLoc = LocaleAuto
)

// Preferences mirrors the on-disk schema. Always populated to a
// usable state by Load (no nil checks needed at call sites).
type Preferences struct {
	loaded         bool
	originalLocale string
	originalTaskUI string
	Version        int    `json:"version"`
	Locale         string `json:"locale"`
	TaskUI         string `json:"taskUI,omitempty"`
}

// IsValidLocale reports whether s is one of the three accepted
// stored values. Anything else (empty string, "zh_CN", "EN", a
// random user-supplied string) is rejected by the configure
// subcommand and HTTP API; the on-disk loader silently coerces
// invalid values to "auto" so a hand-edited bad file doesn't brick
// the CLI.
func IsValidLocale(s string) bool {
	switch s {
	case LocaleAuto, LocaleZhCN, LocaleEnUS:
		return true
	}
	return false
}

// Path returns the absolute path of preferences.json. Useful for
// `one locale` to print where the value lives.
func Path() (string, error) {
	root, err := userdirs.Config()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "preferences.json"), nil
}

// Load reads preferences.json. Missing file → zero-value Preferences
// with Locale="auto". Parse error → returned (caller decides; the
// CLI init path falls back to auto silently).
func Load() (*Preferences, error) {
	paths, err := userdirs.Resolve()
	if err != nil {
		return nil, err
	}
	cache := filepath.Join(paths.State, "preferences-cache.json")
	if _, err = os.Stat(cache); err == nil {
		return LoadAt(cache)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	path, err := Path()
	if err != nil {
		return nil, err
	}
	return LoadAt(path)
}

// CacheAt is a non-authoritative bootstrap hint. It lets help and local tools
// render a saved language without starting or contacting a server.
func CacheAt(root string, p *Preferences) error {
	if p == nil || !IsValidLocale(p.Locale) {
		return i18n.Errorf("preferences.locale_invalid")
	}
	path := filepath.Join(root, "state", "preferences-cache.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return fsutil.WriteAtomic(path, data, 0600)
}

// LoadAt is the testable variant taking an explicit path.
func LoadAt(path string) (*Preferences, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &Preferences{Version: SchemaVersion, Locale: defaultLoc, loaded: true, originalLocale: defaultLoc}, nil
		}
		return nil, err
	}
	var p Preferences
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	// Defensive coercion: a hand-edited bad value falls back to auto
	// rather than propagating into the i18n init path.
	if !IsValidLocale(p.Locale) {
		p.Locale = defaultLoc
	}
	if p.TaskUI != "stream" && p.TaskUI != "tui" {
		p.TaskUI = ""
	}
	if p.Version == 0 {
		p.Version = SchemaVersion
	}

	if p.Version > SchemaVersion {
		return nil, i18n.Errorf("preferences.schema_newer")
	}
	p.loaded = true
	p.originalLocale = p.Locale
	p.originalTaskUI = p.TaskUI
	return &p, nil
}

// Save writes preferences.json atomically (temp file + rename) at
// mode 0600 with parent dir at mode 0700, mirroring profile.Save.
// Deprecated: management preferences are server-owned. Retained for older embedders.
func Save(p *Preferences) error {
	path, err := Path()
	if err != nil {
		return err
	}
	return SaveAt(p, path)
}

// SaveAt is the testable variant.
// Deprecated: use the management API for authoritative preferences.
func SaveAt(p *Preferences, path string) error {
	if p == nil {
		return errors.New(i18n.T("preferences.missing"))
	}
	if !IsValidLocale(p.Locale) {
		return errors.New(i18n.T("preferences.locale_invalid"))
	}
	if p.TaskUI != "" && p.TaskUI != "stream" && p.TaskUI != "tui" {
		return i18n.Errorf("preferences.task_ui_invalid")
	}
	p.Version = SchemaVersion

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	lock := flock.New(path + ".lock")
	locked, err := lock.TryLockContext(ctx, 20*time.Millisecond)
	if err != nil {
		return err
	}
	if !locked {
		return i18n.Errorf("desktop.install_locked")
	}
	defer lock.Unlock()
	fields := map[string]json.RawMessage{}
	current, err := os.ReadFile(path)
	if err == nil {
		if err = json.Unmarshal(current, &fields); err != nil {
			return err
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	var existingVersion int
	if err = json.Unmarshal(fields["version"], &existingVersion); len(fields["version"]) > 0 && err != nil {
		return err
	}
	if existingVersion > SchemaVersion {
		return i18n.Errorf("preferences.schema_newer")
	}
	fields["version"], _ = json.Marshal(SchemaVersion)
	if !p.loaded || p.Locale != p.originalLocale || fields["locale"] == nil {
		fields["locale"], _ = json.Marshal(p.Locale)
	}
	if !p.loaded || p.TaskUI != p.originalTaskUI {
		fields["taskUI"], _ = json.Marshal(p.TaskUI)
	}
	raw, err := json.MarshalIndent(fields, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".preferences-*.json")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		return err
	}
	return fsutil.ReplaceFile(tmpPath, path)
}
