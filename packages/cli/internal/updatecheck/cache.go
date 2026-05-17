package updatecheck

// Cache I/O for the update-check result. Lives at
// $XDG_CACHE_HOME/one/update-check.json (fallback ~/.cache/one/...).
//
// All errors here are silent — this is opportunistic plumbing. If the
// cache can't be read the worst case is "we recheck a bit more often";
// if it can't be written the worst case is "we recheck on every command".
// Neither warrants surfacing a real error to the user.

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// Cache is the on-disk shape of the cached check result. Schema v1 keeps
// the door open for future additions (release notes URL, breaking-change
// flag, channel) without forcing a migration today.
type Cache struct {
	Schema        string    `json:"schema"`
	LastChecked   time.Time `json:"last_checked"`
	LatestVersion string    `json:"latest_version,omitempty"`
}

const cacheSchema = "one-cli/update-check/v1"

// cachePath returns the canonical path to update-check.json. Honors
// XDG_CACHE_HOME (Linux convention; tests can also point it at a tmpdir).
// Returns "" + error only when neither XDG_CACHE_HOME nor $HOME is
// derivable — both should be fatal upstream (we just skip the cache).
func cachePath() (string, error) {
	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		return filepath.Join(xdg, "one", "update-check.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cache", "one", "update-check.json"), nil
}

// loadCache returns the parsed cache, or nil + nil if the file doesn't
// exist (first-run case is not an error). Corrupt JSON returns nil + err
// so callers can distinguish "no data" from "data was lost"; both are
// treated the same way in practice (skip the notification).
func loadCache() (*Cache, error) {
	path, err := cachePath()
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var c Cache
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// saveCache writes the cache atomically (write-temp + rename) so a kill
// mid-write can't leave a half-formed file that the next Load chokes on.
// File mode is whatever os.CreateTemp gave us (0600); contents are a
// version string from a public endpoint, so a tighter mode is harmless.
func saveCache(c *Cache) error {
	if c == nil {
		return errors.New("updatecheck: nil cache")
	}
	c.Schema = cacheSchema
	path, err := cachePath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".update-check-*.json")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // no-op after rename
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
