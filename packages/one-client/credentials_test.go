package oneclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/zalando/go-keyring"
)

type testKeyring struct {
	values map[string]string
	err    error
	calls  int
}

func (k *testKeyring) Get(_, address string) (string, error) {
	k.calls++
	if k.err != nil {
		return "", k.err
	}
	if token, ok := k.values[address]; ok {
		return token, nil
	}
	return "", keyring.ErrNotFound
}
func (k *testKeyring) Set(_, address, token string) error {
	k.calls++
	if k.err != nil {
		return k.err
	}
	if k.values == nil {
		k.values = map[string]string{}
	}
	k.values[address] = token
	return nil
}

func privateClientRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "one")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestFileCredentialsWorkWithoutKeyring(t *testing.T) {
	t.Setenv("ONE_SERVER_URL", "")
	t.Setenv("ONE_SERVER_TOKEN", "")
	ctx := context.Background()
	k := &testKeyring{err: errors.New("Secret Service unavailable")}
	c := &Client{Root: privateClientRoot(t), keyring: k}
	token := "synthetic-server-token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+token {
			t.Error("stored credential was not used")
		}
		_ = json.NewEncoder(w).Encode(Info{APIVersion: 1, Mode: "remote"})
	}))
	defer server.Close()
	if err := c.Add(ctx, "team", server.URL+"/", token); err != nil {
		t.Fatal(err)
	}
	if err := c.Select(ctx, "team", "", ""); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(c.Root, "config", "server-credentials.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatal("credential file is not private")
	}
	config, _ := os.Stat(filepath.Dir(path))
	if runtime.GOOS != "windows" && config.Mode().Perm() != 0700 {
		t.Fatal("credential directory is not private")
	}
	restarted := &Client{Root: c.Root, keyring: k}
	if _, err := restarted.Info(ctx); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ONE_SERVER_URL", server.URL)
	if _, err := restarted.Info(ctx); err != nil {
		t.Fatal(err)
	}
	if k.calls != 0 {
		t.Fatal("file credentials accessed the system keyring")
	}
	before, _ := os.ReadFile(path)
	t.Setenv("ONE_SERVER_TOKEN", "environment-override")
	if conn, err := restarted.connection(ctx, "", ""); err != nil || conn.Token != "environment-override" {
		t.Fatalf("environment token did not take precedence: %v", err)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("environment token changed saved credentials")
	}
	t.Setenv("ONE_SERVER_TOKEN", "")
	profiles, err := c.ReadConnections()
	if err != nil || profiles.Servers["team"].CredentialStore != CredentialFile {
		t.Fatalf("credential store not recorded: %v", err)
	}
	raw, _ := os.ReadFile(filepath.Join(c.Root, "config", "connections.json"))
	if strings.Contains(string(raw), token) {
		t.Fatal("credential leaked into connection metadata")
	}
	if value, err := restarted.remoteToken(server.URL, "", false); err != nil || value != token {
		t.Fatalf("Desktop could not reuse file credentials: %v", err)
	}
}

func TestMissingAndInvalidCredentialsFailWithoutFallback(t *testing.T) {
	t.Setenv("ONE_SERVER_TOKEN", "")
	t.Setenv("ONE_SERVER_URL", "")
	c := &Client{Root: privateClientRoot(t), keyring: &testKeyring{err: errors.New("unavailable")}}
	ctx := context.Background()
	if err := c.AddWithCredentialStore(ctx, "bad", "https://one.example", "token", "unknown"); err == nil {
		t.Fatal("unknown credential store accepted")
	}
	if _, err := os.Stat(filepath.Join(c.Root, "config", "connections.json")); !os.IsNotExist(err) {
		t.Fatal("invalid store changed connection metadata")
	}
	if err := c.Add(ctx, "team", "https://one.example", ""); err != nil {
		t.Fatal(err)
	}
	if err := c.Select(ctx, "team", "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := c.connection(ctx, "", ""); err == nil || !strings.Contains(err.Error(), "--token-stdin") {
		t.Fatal("missing credential did not provide recovery instructions")
	}
	if c.keyring.(*testKeyring).calls != 0 {
		t.Fatal("missing file credential fell back to Secret Service")
	}
}

func TestCredentialStoreMigrationAndDesktopDefaults(t *testing.T) {
	t.Setenv("ONE_SERVER_URL", "")
	t.Setenv("ONE_SERVER_TOKEN", "")
	ctx := context.Background()
	address := "https://one.example"
	k := &testKeyring{values: map[string]string{address: "legacy-token"}}
	c := &Client{Root: privateClientRoot(t), keyring: k}
	if err := c.UpdateConnections(ctx, func(p *Connections) error {
		p.Default = "legacy"
		p.Servers["legacy"] = Profile{URL: address}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if conn, err := c.connection(ctx, "", ""); err != nil || conn.Token != "legacy-token" {
		t.Fatalf("legacy keyring profile broke: %v", err)
	}
	if err := c.AddWithCredentialStore(ctx, "alias", address, "", CredentialFile); err != nil {
		t.Fatal(err)
	}
	k.err = errors.New("keyring is now unavailable")
	if conn, err := c.connection(ctx, "", ""); err != nil || conn.Token != "legacy-token" {
		t.Fatalf("migrated credential needs keyring: %v", err)
	}
	if _, err := c.remoteToken(address, "rotated-token", true); err != nil {
		t.Fatal(err)
	}
	if conn, err := c.connection(ctx, "", ""); err != nil || conn.Token != "rotated-token" {
		t.Fatalf("Desktop rotation left CLI credentials stale: %v", err)
	}
	k.err = nil
	if _, err := c.remoteToken("https://desktop.example", "desktop-token", true); err != nil || k.values["https://desktop.example"] != "desktop-token" {
		t.Fatalf("new Desktop connection did not default to keyring: %v", err)
	}
	if err := c.AddWithCredentialStore(ctx, "alias", address, "", CredentialKeyring); err != nil {
		t.Fatal(err)
	}
	if k.values[address] != "rotated-token" {
		t.Fatal("file-to-keyring migration did not copy the current token")
	}
}

func TestFileCredentialSafety(t *testing.T) {
	for _, name := range []string{"malformed", "newer", "permissions", "directory-permissions", "symlink", "directory-symlink"} {
		t.Run(name, func(t *testing.T) {
			if runtime.GOOS == "windows" && (strings.Contains(name, "permissions") || strings.Contains(name, "symlink")) {
				t.Skip("POSIX file permissions and symlinks")
			}
			c := &Client{Root: privateClientRoot(t), keyring: &testKeyring{err: errors.New("unavailable")}}
			ctx := context.Background()
			address := "https://one.example"
			if err := c.Add(ctx, "team", address, "original-token"); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(c.Root, "config", "server-credentials.json")
			switch name {
			case "malformed":
				_ = os.WriteFile(path, []byte(`{"tokens":"private-value"`), 0600)
			case "newer":
				_ = os.WriteFile(path, []byte(`{"version":999,"tokens":{}}`), 0600)
			case "permissions":
				_ = os.Chmod(path, 0644)
			case "directory-permissions":
				_ = os.Chmod(c.Root, 0770)
			case "symlink":
				target := filepath.Join(t.TempDir(), "target")
				_ = os.WriteFile(target, []byte("private-value"), 0600)
				_ = os.Remove(path)
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			case "directory-symlink":
				config := filepath.Join(c.Root, "config")
				moved := filepath.Join(c.Root, "original-config")
				_ = os.Rename(config, moved)
				if err := os.Symlink(moved, config); err != nil {
					t.Fatal(err)
				}
			}
			before, _ := os.ReadFile(path)
			if _, err := c.remoteToken(address, "", false); err == nil || strings.Contains(err.Error(), "private-value") {
				t.Fatal("unsafe credential file was read or its contents leaked")
			}
			if err := c.Add(ctx, "team", address, "replacement"); err == nil {
				t.Fatal("unsafe credential file was silently overwritten")
			}
			after, _ := os.ReadFile(path)
			if string(before) != string(after) {
				t.Fatal("existing credentials were changed on failure")
			}
		})
	}
}

func TestConcurrentCredentialUpdates(t *testing.T) {
	c := &Client{Root: privateClientRoot(t)}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := c.Add(context.Background(), fmt.Sprintf("team-%d", i), fmt.Sprintf("https://server-%d.example", i), fmt.Sprintf("token-%d", i)); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	for i := 0; i < 8; i++ {
		value, err := c.remoteToken(fmt.Sprintf("https://server-%d.example", i), "", false)
		if err != nil || value != fmt.Sprintf("token-%d", i) {
			t.Fatalf("concurrent update lost a credential: %v", err)
		}
	}
}
