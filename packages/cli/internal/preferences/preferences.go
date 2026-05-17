// Package preferences owns the user-global preference file at
// ~/.config/one/preferences.json (XDG-aware; honours XDG_CONFIG_HOME
// just like internal/profile).
//
// This is the home for cross-workspace UI preferences that have no
// concept of (domain, backend) — today just Locale, in future
// possibly theme, telemetry opt-in, default editor, etc. Profile
// configuration (per-(domain, backend) endpoint + credentials) lives
// in internal/profile and is intentionally kept separate: profile
// state has its own AWS-CLI-style two-file split for credential
// handling that we do not want or need here.
//
// File layout:
//
//	~/.config/one/preferences.json  — JSON, mode 0600
//
// Schema:
//
//	{
//	  "version": 1,
//	  "locale":  "auto" | "zh-CN" | "en-US"
//	}
//
// Missing file is not an error — Load returns zero values
// (Locale="auto") so first-run callers don't need to special-case it.
package preferences

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
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
	Version int    `json:"version"`
	Locale  string `json:"locale"`
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

// configRoot returns ~/.config/one (XDG-aware). Mirrors
// profile.configRoot — kept duplicated rather than re-exported so
// the preferences package doesn't drag in the profile dependency
// graph.
func configRoot() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "one"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "one"), nil
}

// Path returns the absolute path of preferences.json. Useful for
// `one configure locale` to print where the value lives.
func Path() (string, error) {
	root, err := configRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "preferences.json"), nil
}

// Load reads preferences.json. Missing file → zero-value Preferences
// with Locale="auto". Parse error → returned (caller decides; the
// CLI init path falls back to auto silently).
func Load() (*Preferences, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	return LoadAt(path)
}

// LoadAt is the testable variant taking an explicit path.
func LoadAt(path string) (*Preferences, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &Preferences{Version: SchemaVersion, Locale: defaultLoc}, nil
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
	if p.Version == 0 {
		p.Version = SchemaVersion
	}
	return &p, nil
}

// Save writes preferences.json atomically (temp file + rename) at
// mode 0600 with parent dir at mode 0700, mirroring profile.Save.
func Save(p *Preferences) error {
	path, err := Path()
	if err != nil {
		return err
	}
	return SaveAt(p, path)
}

// SaveAt is the testable variant.
func SaveAt(p *Preferences, path string) error {
	if p == nil {
		return errors.New("preferences: nil")
	}
	if !IsValidLocale(p.Locale) {
		return errors.New("preferences: invalid locale; expected auto | zh-CN | en-US")
	}
	p.Version = SchemaVersion

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(p, "", "  ")
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
	return os.Rename(tmpPath, path)
}
