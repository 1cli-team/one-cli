package profile

// Locks the Upsert vs Add semantic split: Upsert silently overwrites
// while Add errors PROFILE_ALREADY_EXISTS. Setup commands rely on
// Upsert; the legacy `<domain> profile add` CRUD surface keeps
// strict-add. Also covers the per-(domain, backend) section split
// plus the AWS-style two-file split: same name across different
// backends can coexist; secrets land in credentials.json, never in
// config.json.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func withIsolatedConfig(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("HOME", tmp)
	return tmp
}

func cfgPaths(tmp string) (cfg, creds string) {
	return filepath.Join(tmp, "one", "config.json"),
		filepath.Join(tmp, "one", "credentials.json")
}

// First Upsert creates a fresh entry and reports updated=false. The
// "first profile becomes default" auto-default rule fires regardless of
// the setDefault flag.
func TestUpsert_FreshEntry(t *testing.T) {
	withIsolatedConfig(t)
	updated, err := Upsert(DomainContainer, "docker", "acr-prod", Profile{
		Backend: "docker",
		Container: &ContainerProfile{
			Registry: "registry.example.com",
			Credentials: &ContainerCredentials{
				Username: "u", Password: "p",
			},
		},
	}, false)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if updated {
		t.Errorf("first add: updated=true, want false")
	}
	cfg, _, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.ContainerDocker.Default != "acr-prod" {
		t.Errorf("auto-default rule failed: default=%q", cfg.ContainerDocker.Default)
	}
	if cfg.ContainerDocker.Profiles["acr-prod"].Registry != "registry.example.com" {
		t.Errorf("registry not persisted: %#v", cfg.ContainerDocker.Profiles["acr-prod"])
	}
	// Credentials must come back via Load → mergeCredentials.
	if cred := cfg.ContainerDocker.Profiles["acr-prod"].Credentials; cred == nil ||
		cred.Username != "u" || cred.Password != "p" {
		t.Errorf("credentials not merged from credentials.json: %+v", cred)
	}
}

// Same name twice updates in place, returns updated=true, leaves the
// default pointer where it was.
func TestUpsert_OverwriteExisting(t *testing.T) {
	withIsolatedConfig(t)
	first := Profile{
		Backend: "infisical",
		Infisical: &InfisicalProfile{
			SiteURL: "https://app.infisical.com",
			Credentials: &InfisicalCredentials{
				ClientID: "cid-1", ClientSecret: "cs-1",
			},
		},
	}
	if _, err := Upsert(DomainEnv, "infisical", "work", first, false); err != nil {
		t.Fatalf("first: %v", err)
	}
	second := first
	second.Infisical = &InfisicalProfile{
		SiteURL: "https://app.infisical.com",
		Credentials: &InfisicalCredentials{
			ClientID: "cid-1", ClientSecret: "cs-2-rotated",
		},
	}
	updated, err := Upsert(DomainEnv, "infisical", "work", second, false)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if !updated {
		t.Errorf("second upsert: updated=false, want true")
	}
	cfg, _, _ := Load()
	if got := cfg.EnvInfisical.Profiles["work"].Credentials.ClientSecret; got != "cs-2-rotated" {
		t.Errorf("clientSecret not rotated: got %q", got)
	}
}

// Same name in different backends doesn't collide because sections are split
// pins each profile to its own (domain, backend) section.
func TestUpsert_NameAcrossBackendsDoesNotCollide(t *testing.T) {
	withIsolatedConfig(t)
	if _, err := Upsert(DomainEnv, "infisical", "work", Profile{
		Backend: "infisical",
		Infisical: &InfisicalProfile{
			SiteURL:     "https://app.infisical.com",
			Credentials: &InfisicalCredentials{ClientID: "x", ClientSecret: "y"},
		},
	}, false); err != nil {
		t.Fatalf("infisical: %v", err)
	}
	if _, err := Upsert(DomainContainer, "docker", "work", Profile{
		Backend: "docker",
		Container: &ContainerProfile{
			Registry:    "registry.example.com",
			Credentials: &ContainerCredentials{Username: "u", Password: "p"},
		},
	}, false); err != nil {
		t.Fatalf("docker: %v", err)
	}
	cfg, _, _ := Load()
	if cfg.EnvInfisical.Profiles["work"].SiteURL != "https://app.infisical.com" {
		t.Errorf("infisical work lost")
	}
	if cfg.ContainerDocker.Profiles["work"].Registry != "registry.example.com" {
		t.Errorf("docker work lost")
	}
}

// SaveAt + LoadAt round-trip Container profile shape, including the
// AWS-style file split.
func TestContainerProfile_Roundtrip(t *testing.T) {
	tmp := withIsolatedConfig(t)
	cfgPath, credPath := cfgPaths(tmp)
	cfg := &Config{Version: SchemaVersion}
	cfg.ContainerDocker.Profiles = map[string]ContainerProfile{
		"acr-prod": {
			Registry: "registry.example.com",
			Credentials: &ContainerCredentials{
				Username: "ram-ak",
				Password: "ram-secret",
			},
		},
	}
	cfg.ContainerDocker.Default = "acr-prod"
	if err := SaveAt(cfg, cfgPath, credPath); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, _, err := LoadAt(cfgPath, credPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	cp := got.ContainerDocker.Profiles["acr-prod"]
	if cp.Registry != "registry.example.com" {
		t.Errorf("fields lost: %#v", cp)
	}
	if cp.Credentials == nil || cp.Credentials.Username != "ram-ak" || cp.Credentials.Password != "ram-secret" {
		t.Errorf("credentials lost: %#v", cp.Credentials)
	}
	for _, p := range []string{cfgPath, credPath} {
		st, err := os.Stat(p)
		if err != nil {
			t.Fatalf("stat %s: %v", p, err)
		}
		if mode := st.Mode().Perm(); mode != 0o600 {
			t.Errorf("file mode %s: got %o want 0600", p, mode)
		}
	}
	// config.json must NOT contain the password.
	cfgRaw, _ := os.ReadFile(cfgPath)
	if strings.Contains(string(cfgRaw), "ram-secret") {
		t.Errorf("password leaked into config.json:\n%s", cfgRaw)
	}
	if strings.Contains(string(cfgRaw), "\"credentials\"") {
		t.Errorf("credentials key present in config.json (should be empty):\n%s", cfgRaw)
	}
	// credentials.json must contain the password.
	credRaw, _ := os.ReadFile(credPath)
	if !strings.Contains(string(credRaw), "ram-secret") {
		t.Errorf("password missing from credentials.json:\n%s", credRaw)
	}
}

// Both files coexist independently: Save writes config.json +
// credentials.json. Load tolerates either side missing — all missing
// returns an empty Config / CredentialsFile pair with the current
// schema version.
func TestLoad_MissingFiles(t *testing.T) {
	tmp := withIsolatedConfig(t)
	cfgPath, credPath := cfgPaths(tmp)

	cfg, creds, err := LoadAt(cfgPath, credPath)
	if err != nil {
		t.Fatalf("fresh: %v", err)
	}
	if cfg.Version != SchemaVersion || creds.Version != SchemaVersion {
		t.Errorf("fresh load did not init Version: cfg=%d creds=%d", cfg.Version, creds.Version)
	}
}

// Saving with no profiles still emits {"version":<schema>} in both
// files, not raw `null` or empty objects. Keeps Load happy on
// round-trip. Compares against SchemaVersion so the test follows the
// schema bump without hand edits.
func TestSave_EmptyConfigShape(t *testing.T) {
	tmp := withIsolatedConfig(t)
	cfgPath, credPath := cfgPaths(tmp)
	if err := SaveAt(&Config{Version: SchemaVersion}, cfgPath, credPath); err != nil {
		t.Fatalf("save: %v", err)
	}
	wantVersion := fmt.Sprintf("%d", SchemaVersion)
	for _, p := range []string{cfgPath, credPath} {
		raw, _ := os.ReadFile(p)
		var probe map[string]json.RawMessage
		if err := json.Unmarshal(raw, &probe); err != nil {
			t.Fatalf("parse %s: %v", p, err)
		}
		if string(probe["version"]) != wantVersion {
			t.Errorf("%s version key missing or wrong: %s", p, raw)
		}
	}
}

// Legacy docs used an `active` pointer before the current `default`
// pointer. Loading one must surface PROFILE_VERSION_UNSUPPORTED rather
// than risking default loss on the next save.
func TestLoad_RejectsLegacyActiveSchema(t *testing.T) {
	tmp := withIsolatedConfig(t)
	cfgPath, credPath := cfgPaths(tmp)
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	legacyDoc := `{
		"version": 0,
		"env/infisical": {
			"active": "work",
			"profiles": {
				"work": {"siteUrl": "https://app.infisical.com"}
			}
		}
	}`
	if err := os.WriteFile(cfgPath, []byte(legacyDoc), 0o600); err != nil {
		t.Fatalf("write legacy cfg: %v", err)
	}
	if err := os.WriteFile(credPath, []byte(`{"version":0}`), 0o600); err != nil {
		t.Fatalf("write legacy creds: %v", err)
	}
	_, _, err := LoadAt(cfgPath, credPath)
	if err == nil {
		t.Fatal("expected PROFILE_VERSION_UNSUPPORTED for legacy cfg")
	}
	if cliErr, ok := err.(interface{ ErrorCode() string }); !ok || cliErr.ErrorCode() != "PROFILE_VERSION_UNSUPPORTED" {
		t.Fatalf("error = %v, want PROFILE_VERSION_UNSUPPORTED", err)
	}
}

// A document with a version OLDER than MinSupportedVersion must surface PROFILE_VERSION_UNSUPPORTED
// rather than be silently parsed.
func TestLoad_RejectsBelowMinSupportedVersion(t *testing.T) {
	tmp := withIsolatedConfig(t)
	cfgPath, credPath := cfgPaths(tmp)
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(cfgPath, []byte(`{"version":0}`), 0o600); err != nil {
		t.Fatalf("write old cfg: %v", err)
	}
	_, _, err := LoadAt(cfgPath, credPath)
	if err == nil {
		t.Fatal("expected PROFILE_VERSION_UNSUPPORTED for old cfg")
	}
	if cliErr, ok := err.(interface{ ErrorCode() string }); !ok || cliErr.ErrorCode() != "PROFILE_VERSION_UNSUPPORTED" {
		t.Fatalf("error = %v, want PROFILE_VERSION_UNSUPPORTED", err)
	}
}

// A document with a version NEWER than this binary's SchemaVersion
// must also be rejected — running an older binary against a config
// the user upgraded to a newer schema is a real footgun (silent data
// loss on save), so we surface it loudly.
func TestLoad_RejectsAboveSchemaVersion(t *testing.T) {
	tmp := withIsolatedConfig(t)
	cfgPath, credPath := cfgPaths(tmp)
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	future := fmt.Sprintf(`{"version":%d}`, SchemaVersion+1)
	if err := os.WriteFile(cfgPath, []byte(future), 0o600); err != nil {
		t.Fatalf("write future cfg: %v", err)
	}
	_, _, err := LoadAt(cfgPath, credPath)
	if err == nil {
		t.Fatal("expected PROFILE_VERSION_UNSUPPORTED for future schema")
	}
	if cliErr, ok := err.(interface{ ErrorCode() string }); !ok || cliErr.ErrorCode() != "PROFILE_VERSION_UNSUPPORTED" {
		t.Fatalf("error = %v, want PROFILE_VERSION_UNSUPPORTED", err)
	}
}

// Add (strict mode) refuses overwrite within the same section.
func TestAdd_RejectsDuplicate(t *testing.T) {
	withIsolatedConfig(t)
	mk := func() Profile {
		return Profile{
			Backend: "infisical",
			Infisical: &InfisicalProfile{
				SiteURL: "https://app.infisical.com",
				Credentials: &InfisicalCredentials{
					ClientID: "x", ClientSecret: "y",
				},
			},
		}
	}
	if err := Add(DomainEnv, "infisical", "work", mk(), false); err != nil {
		t.Fatalf("first add: %v", err)
	}
	err := Add(DomainEnv, "infisical", "work", mk(), false)
	if err == nil {
		t.Fatalf("expected PROFILE_ALREADY_EXISTS")
	}
	if !strings.Contains(err.Error(), "已存在") {
		t.Errorf("unexpected error: %v", err)
	}
}

// Remove deletes from both files and clears the cache file.
func TestRemove_ClearsCache(t *testing.T) {
	withIsolatedConfig(t)
	if _, err := Upsert(DomainEnv, "infisical", "work", Profile{
		Backend: "infisical",
		Infisical: &InfisicalProfile{
			SiteURL:     "https://app.infisical.com",
			Credentials: &InfisicalCredentials{ClientID: "x", ClientSecret: "y"},
		},
	}, false); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// Plant a fake cache entry.
	if err := WriteCache(DomainEnv, "infisical", "work", &CacheEntry{
		Token:     "stale-token",
		ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("write cache: %v", err)
	}
	if err := Remove(DomainEnv, "infisical", "work"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	cachePath, _ := CachePath(DomainEnv, "infisical", "work")
	if _, err := os.Stat(cachePath); !os.IsNotExist(err) {
		t.Errorf("cache file should be gone after remove, stat err=%v", err)
	}
}
