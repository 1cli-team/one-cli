package profile

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Round-trip a non-expired entry through Write/Read.
func TestCache_WriteRead(t *testing.T) {
	withIsolatedConfig(t)
	now := time.Now().UTC()
	entry := &CacheEntry{
		Token:     "abc.def.ghi",
		TokenType: "Bearer",
		ExpiresAt: now.Add(2 * time.Hour),
		SavedAt:   now,
	}
	if err := WriteCache(DomainEnv, "infisical", "work", entry); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := ReadCache(DomainEnv, "infisical", "work")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got == nil {
		t.Fatalf("read returned nil; want hit")
	}
	if got.Token != entry.Token || got.TokenType != entry.TokenType {
		t.Errorf("round-trip lost fields: %+v", got)
	}
}

// Expired entries return (nil, nil) so callers fall through to login.
func TestCache_ExpiredReturnsMiss(t *testing.T) {
	withIsolatedConfig(t)
	if err := WriteCache(DomainEnv, "infisical", "work", &CacheEntry{
		Token:     "expired-token",
		ExpiresAt: time.Now().Add(-time.Hour),
		SavedAt:   time.Now().Add(-2 * time.Hour),
	}); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := ReadCache(DomainEnv, "infisical", "work")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got != nil {
		t.Errorf("expected expired→nil, got %+v", got)
	}
}

// Entry one second from expiring is treated as expired (60s skew buffer).
func TestCache_NearExpiryWithinSkewMisses(t *testing.T) {
	withIsolatedConfig(t)
	if err := WriteCache(DomainEnv, "infisical", "work", &CacheEntry{
		Token:     "almost",
		ExpiresAt: time.Now().Add(30 * time.Second),
	}); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, _ := ReadCache(DomainEnv, "infisical", "work")
	if got != nil {
		t.Errorf("near-expiry should be treated as miss: %+v", got)
	}
}

// Cache files must be 0600.
func TestCache_FileMode(t *testing.T) {
	withIsolatedConfig(t)
	if err := WriteCache(DomainEnv, "infisical", "work", &CacheEntry{
		Token:     "x",
		ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("write: %v", err)
	}
	path, _ := CachePath(DomainEnv, "infisical", "work")
	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if mode := st.Mode().Perm(); mode != 0o600 {
		t.Errorf("mode: got %o want 0600", mode)
	}
}

// A corrupted JSON file is treated as a miss, not an error.
func TestCache_CorruptedFileTreatedAsMiss(t *testing.T) {
	withIsolatedConfig(t)
	path, _ := CachePath(DomainEnv, "infisical", "work")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("not json"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := ReadCache(DomainEnv, "infisical", "work")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got != nil {
		t.Errorf("corrupted should miss; got %+v", got)
	}
}

// ClearCache is best-effort: removing a non-existent file is not an error.
func TestCache_ClearIdempotent(t *testing.T) {
	withIsolatedConfig(t)
	if err := ClearCache(DomainEnv, "infisical", "never-existed"); err != nil {
		t.Errorf("clear on missing: %v", err)
	}
	if err := WriteCache(DomainEnv, "infisical", "work", &CacheEntry{
		Token:     "x",
		ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := ClearCache(DomainEnv, "infisical", "work"); err != nil {
		t.Errorf("clear: %v", err)
	}
	got, _ := ReadCache(DomainEnv, "infisical", "work")
	if got != nil {
		t.Errorf("entry survived clear: %+v", got)
	}
}
