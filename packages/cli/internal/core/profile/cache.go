package profile

// cache.go is the short-lived-token cache for backends that exchange
// long-term credentials (file-source AKID/secret) for an OIDC-style
// session token. Today only the Infisical Universal-Auth login uses
// it; the layer is generic so SSO / `credential_process` integrations
// can reuse it later.
//
// Layout: ~/.config/one/cache/<domain>/<backend>/<profile>.json
// Mode: 0600 per file, 0700 for parent dirs.
//
// Persistent (long-term) credentials live in ~/.config/one/credentials.json
// — the cache is intentionally a separate directory so it can be wiped
// without losing the user's profile config (`rm -rf ~/.config/one/cache`).

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// cacheClockSkew is the buffer subtracted from a cache entry's
// ExpiresAt before deciding it's still good. 60s avoids returning a
// token that expires mid-flight.
const cacheClockSkew = 60 * time.Second

// CacheEntry is one cached short-lived token. Wire format intentionally
// minimal — additional fields (e.g. refresh_token, issuer) can be
// added later without breaking back-compat because unknown fields are
// ignored on decode.
type CacheEntry struct {
	Token     string    `json:"token"`
	TokenType string    `json:"tokenType,omitempty"`
	ExpiresAt time.Time `json:"expiresAt"`
	SavedAt   time.Time `json:"savedAt"`
}

// IsExpired reports whether the entry should not be reused given the
// cacheClockSkew buffer.
func (e *CacheEntry) IsExpired(now time.Time) bool {
	if e == nil {
		return true
	}
	return now.Add(cacheClockSkew).After(e.ExpiresAt)
}

// CachePath returns the cache file path for one (domain, backend,
// profile) triple. Does not create the file.
func CachePath(domain Domain, backend, name string) (string, error) {
	root, err := CacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, string(domain), backend, name+".json"), nil
}

// ReadCache returns the parsed cache entry or nil when:
//   - the file does not exist;
//   - the file exists but fails to parse;
//   - the entry has expired (per IsExpired).
//
// All three "no usable token" conditions are conflated into (nil, nil)
// so callers always know what to do: fall through to a fresh login.
// Real I/O errors (permission, disk) still surface as non-nil err.
func ReadCache(domain Domain, backend, name string) (*CacheEntry, error) {
	path, err := CachePath(domain, backend, name)
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
	var entry CacheEntry
	if err := json.Unmarshal(raw, &entry); err != nil {
		// Corrupted cache — pretend it's not there.
		return nil, nil
	}
	if entry.IsExpired(time.Now().UTC()) {
		return nil, nil
	}
	return &entry, nil
}

// WriteCache atomically persists entry as the cache for (domain,
// backend, name). Creates parent dirs at 0700 and writes the file at
// 0600.
func WriteCache(domain Domain, backend, name string, entry *CacheEntry) error {
	if entry == nil {
		return errors.New("profile: nil cache entry")
	}
	path, err := CachePath(domain, backend, name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return atomicWrite(entry, path)
}

// ClearCache deletes the cache file for (domain, backend, name).
// Missing file is not an error — the function is meant to be called
// best-effort during profile remove / login failure paths.
func ClearCache(domain Domain, backend, name string) error {
	path, err := CachePath(domain, backend, name)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
