package profile

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestValidateNameContract(t *testing.T) {
	for _, name := range []string{
		"work",
		"acr-prod",
		"personal_2",
		"A",
		"a" + strings.Repeat("b", maxProfileNameLength-1),
	} {
		if err := ValidateName(name); err != nil {
			t.Errorf("ValidateName(%q): %v", name, err)
		}
	}

	for _, name := range []string{
		"",
		" work",
		"work ",
		"work\n",
		"../work",
		"..",
		"team/work",
		`team\work`,
		".hidden",
		"prod.profile",
		"-work",
		"_work",
		"测试",
		"a" + strings.Repeat("b", maxProfileNameLength),
	} {
		err := ValidateName(name)
		if err == nil {
			t.Errorf("ValidateName(%q) unexpectedly succeeded", name)
			continue
		}
		var coded interface{ ErrorCode() string }
		if !errors.As(err, &coded) || coded.ErrorCode() != "PROFILE_BACKEND_INVALID" {
			t.Errorf("ValidateName(%q) code = %v, want PROFILE_BACKEND_INVALID", name, err)
		}
	}
}

func TestProfileMutationBoundariesRejectUnsafeNameBeforeWriting(t *testing.T) {
	tmp := withIsolatedConfig(t)
	unsafeName := "../../../../outside"
	value := Profile{
		Backend: "infisical",
		Infisical: &InfisicalProfile{
			SiteURL: "https://app.infisical.com",
			Credentials: &InfisicalCredentials{
				ClientID: "client", ClientSecret: "secret",
			},
		},
	}

	operations := map[string]func() error{
		"add": func() error {
			return Add(DomainEnv, "infisical", unsafeName, value, false)
		},
		"upsert": func() error {
			_, err := Upsert(DomainEnv, "infisical", unsafeName, value, false)
			return err
		},
		"remove": func() error {
			return Remove(DomainEnv, "infisical", unsafeName)
		},
		"set default": func() error {
			return SetDefault(DomainEnv, "infisical", unsafeName)
		},
		"bind workspace": func() error {
			return BindWorkspaceProfile(
				"workspace-id", "workspace", tmp, "api",
				DomainEnv, "infisical", unsafeName,
			)
		},
	}

	for name, operation := range operations {
		t.Run(name, func(t *testing.T) {
			if err := operation(); err == nil {
				t.Fatal("unsafe profile name unexpectedly succeeded")
			}
		})
	}

	configPath, err := ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath: %v", err)
	}
	credentialsPath, err := CredentialsPath()
	if err != nil {
		t.Fatalf("CredentialsPath: %v", err)
	}
	for _, path := range []string{configPath, credentialsPath} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("unsafe mutation created %s; stat error = %v", path, err)
		}
	}
}

func TestCacheBoundariesRejectTraversalWithoutTouchingOutsideFile(t *testing.T) {
	tmp := withIsolatedConfig(t)
	outsidePath := filepath.Join(tmp, "sentinel.json")
	want := []byte("do-not-touch")
	if err := os.WriteFile(outsidePath, want, 0o600); err != nil {
		t.Fatalf("write sentinel: %v", err)
	}

	// From ~/.config/one/cache/env/infisical, four parent components would
	// resolve to XDG_CONFIG_HOME and reach sentinel.json without validation.
	unsafeName := "../../../../sentinel"
	if path, err := CachePath(DomainEnv, "infisical", unsafeName); err == nil {
		t.Fatalf("CachePath returned unsafe path %q", path)
	}
	if entry, err := ReadCache(DomainEnv, "infisical", unsafeName); err == nil || entry != nil {
		t.Fatalf("ReadCache = (%+v, %v), want validation error", entry, err)
	}
	if err := WriteCache(DomainEnv, "infisical", unsafeName, &CacheEntry{
		Token:     "overwrite",
		ExpiresAt: time.Now().Add(time.Hour),
	}); err == nil {
		t.Fatal("WriteCache accepted traversal name")
	}
	if err := ClearCache(DomainEnv, "infisical", unsafeName); err == nil {
		t.Fatal("ClearCache accepted traversal name")
	}

	got, err := os.ReadFile(outsidePath)
	if err != nil {
		t.Fatalf("sentinel was removed: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("sentinel changed: got %q, want %q", got, want)
	}
}
