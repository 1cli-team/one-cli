package infisicalsession_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	cliErrors "github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	session "github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/infisicalsession"
)

func isolateSession(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ONE_HOME", filepath.Join(home, "one"))
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path="+filepath.Join(home, "no-keyring"))
	path, err := session.ConfigPath("session.json")
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadLocalSession(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
		missing bool
		expired bool
	}{
		{name: "absent", missing: true},
		{name: "invalid JSON", content: "{", missing: true},
		{name: "missing token", content: `{}`, missing: true},
		{name: "expired", content: `{"token":"test-token","expiresAt":"2000-01-01T00:00:00Z"}`, expired: true},
		{name: "active", content: `{"token":"test-token","siteUrl":"https://example.com","userId":"test-user","expiresAt":"2099-01-01T00:00:00Z"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := isolateSession(t)
			if tc.content != "" {
				if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(tc.content), 0600); err != nil {
					t.Fatal(err)
				}
			}
			s, err := session.Load()
			if tc.missing {
				var coded interface{ ErrorCode() string }
				if !errors.As(err, &coded) || coded.ErrorCode() != string(cliErrors.INFISICAL_AUTH_MISSING) {
					t.Fatalf("Load error = %v; want missing authentication", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if s.Token != "test-token" || s.Expired != tc.expired || s.LoggedIn == tc.expired {
				t.Fatal("loaded session does not match the saved credentials and expiry")
			}
			_, err = session.Require()
			if (err != nil) != tc.expired {
				t.Fatalf("Require error = %v; expired = %v", err, tc.expired)
			}
		})
	}
}

func loginAttempt(t *testing.T, beforeSubmit func()) (*session.Attempt, string) {
	t.Helper()
	claims, err := json.Marshal(map[string]any{"userId": "test-user", "organizationId": "test-org", "exp": time.Now().Add(time.Hour).Unix()})
	if err != nil {
		t.Fatal(err)
	}
	token := "test." + base64.RawURLEncoding.EncodeToString(claims) + ".signature"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/auth/checkAuth" || r.Header.Get("Authorization") != "Bearer "+token {
			http.Error(w, "invalid verification request", http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	attempt, err := session.Start(ctx, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { attempt.Cancel(); _, _ = attempt.Wait() })
	if beforeSubmit != nil {
		beforeSubmit()
	}
	credentials, err := json.Marshal(map[string]string{"JTWToken": token, "email": "test@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if err := attempt.SubmitBrowserToken(base64.StdEncoding.EncodeToString(credentials)); err != nil {
		t.Fatal(err)
	}
	return attempt, token
}

func TestLocalSessionLifecycle(t *testing.T) {
	path := isolateSession(t)
	attempt, token := loginAttempt(t, nil)
	if _, err := attempt.Wait(); err != nil {
		t.Fatal(err)
	}
	loaded, err := session.Require()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Token != token || loaded.UserID != "test-user" || loaded.Email != "test@example.com" {
		t.Fatal("login did not persist the verified session")
	}
	for _, p := range []string{path, filepath.Dir(path)} {
		info, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		want := fs.FileMode(0600)
		if info.IsDir() {
			want = 0700
		}
		if runtime.GOOS != "windows" && info.Mode().Perm() != want {
			t.Errorf("%s permissions = %o; want %o", p, info.Mode().Perm(), want)
		}
	}
	info, err := session.Status()
	if err != nil {
		t.Fatal(err)
	}
	status, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(status), token) || strings.Contains(string(status), `"token"`) {
		t.Fatal("status exposes the session token")
	}
	committed := false
	if err := session.WithUnchanged(loaded, func() error { committed = true; return nil }); err != nil || !committed {
		t.Fatalf("commit with current session failed: %v", err)
	}
	for range 2 {
		if err := session.Logout(); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("session file after logout: %v", err)
	}
	info, err = session.Status()
	if err != nil || info.LoggedIn {
		t.Fatalf("status after logout: logged in = %v, error = %v", info.LoggedIn, err)
	}
	if err := session.WithUnchanged(loaded, func() error { t.Error("commit accepted a logged-out session"); return nil }); err == nil {
		t.Error("expected stale session commit to fail")
	}
	files, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".session-*"))
	if err != nil || len(files) != 0 {
		t.Fatalf("temporary credential files remain: %v (error: %v)", files, err)
	}
}

func TestLoginReplacesLocalSession(t *testing.T) {
	path := isolateSession(t)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"token":"expired-token"}`), 0644); err != nil {
		t.Fatal(err)
	}
	attempt, token := loginAttempt(t, nil)
	if _, err := attempt.Wait(); err != nil {
		t.Fatal(err)
	}
	loaded, err := session.Require()
	if err != nil || loaded.Token != token {
		t.Fatalf("could not reload replacement credentials: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Errorf("replacement file permissions = %o; want 600", info.Mode().Perm())
	}
}

func TestLocalSessionStorageErrors(t *testing.T) {
	originalLocale := i18n.Active()
	t.Cleanup(func() { _ = i18n.Init(originalLocale) })
	for _, locale := range []string{"en-US", "zh-CN"} {
		t.Run(locale, func(t *testing.T) {
			path := isolateSession(t)
			if err := i18n.Init(locale); err != nil {
				t.Fatal(err)
			}
			marker := filepath.Join(path, "keep")
			attempt, token := loginAttempt(t, func() {
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(marker, []byte("keep"), 0600); err != nil {
					t.Fatal(err)
				}
			})
			_, err := attempt.Wait()
			if err == nil || errors.Unwrap(err) == nil || !strings.HasPrefix(err.Error(), strings.TrimSuffix(i18n.T("auth.store_write_failed"), "%w")) || strings.Contains(err.Error(), token) {
				t.Fatalf("expected localized storage error without credentials, got %v", err)
			}
			if _, err := os.Stat(marker); err != nil {
				t.Fatalf("failed save modified the existing target: %v", err)
			}
			files, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".session-*"))
			if err != nil || len(files) != 0 {
				t.Fatalf("failed save left temporary credentials: %v (error: %v)", files, err)
			}
			_, err = session.Load()
			var pathErr *os.PathError
			if !errors.As(err, &pathErr) || pathErr.Path != path || !strings.HasPrefix(err.Error(), strings.TrimSuffix(i18n.T("auth.store_read_failed"), "%w")) {
				t.Fatalf("expected localized read error preserving the path and cause, got %v", err)
			}
			if err := session.Logout(); err == nil || errors.Unwrap(err) == nil || !strings.HasPrefix(err.Error(), strings.TrimSuffix(i18n.T("auth.store_delete_failed"), "%w")) {
				t.Fatalf("expected localized logout error, got %v", err)
			}
		})
	}
}

func TestSessionConfigLocation(t *testing.T) {
	for _, override := range []bool{false, true} {
		t.Run(map[bool]string{false: "default home", true: "ONE_HOME"}[override], func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("ONE_HOME", "")
			root := filepath.Join(home, ".one")
			if override {
				root = filepath.Join(home, "custom")
				t.Setenv("ONE_HOME", root)
			}
			path, err := session.ConfigPath("session.json")
			if err != nil || path != filepath.Join(root, "config", "session.json") {
				t.Fatalf("ConfigPath = %q, error = %v", path, err)
			}
		})
	}
}
