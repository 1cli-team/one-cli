package cloudflare

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/output"
)

func TestPreflightD1DatabaseBindingsPassesExistingDatabase(t *testing.T) {
	dir := seedD1WranglerConfig(t, `db-ok`, `my-db`)
	withCloudflareAPIServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/client/v4/accounts/acct/d1/database/db-ok" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Fatalf("Authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"result":{"uuid":"db-ok","name":"my-db"},"errors":[]}`))
	}))

	if err := preflightD1DatabaseBindings(context.Background(), dir, "tok", "acct"); err != nil {
		t.Fatalf("preflightD1DatabaseBindings: %v", err)
	}
}

func TestPreflightD1DatabaseBindingsRejectsMissingDatabase(t *testing.T) {
	dir := seedD1WranglerConfig(t, `wrong-id`, `my-db`)
	withCloudflareAPIServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"success":false,"result":null,"errors":[{"code":1001,"message":"database not found"}]}`))
	}))

	err := preflightD1DatabaseBindings(context.Background(), dir, "tok", "acct")
	if err == nil {
		t.Fatal("expected D1 preflight error")
	}
	outErr, ok := err.(*output.Error)
	if !ok {
		t.Fatalf("error type = %T, want *output.Error", err)
	}
	if outErr.Code != "CLOUDFLARE_DEPLOY_FAILED" {
		t.Fatalf("code = %s", outErr.Code)
	}
	if outErr.Context["database_id"] != "wrong-id" {
		t.Fatalf("database_id context missing: %v", outErr.Context)
	}
	if !hasRemediationAction(outErr.Remediation, "check-d1-database-id") {
		t.Fatalf("D1 remediation missing: %+v", outErr.Remediation)
	}
}

func TestPreflightD1DatabaseBindingsResolvesSingleAccount(t *testing.T) {
	dir := seedD1WranglerConfig(t, `db-ok`, `my-db`)
	withCloudflareAPIServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/client/v4/accounts":
			_, _ = w.Write([]byte(`{"success":true,"result":[{"id":"acct","name":"prod"}],"errors":[]}`))
		case "/client/v4/accounts/acct/d1/database/db-ok":
			_, _ = w.Write([]byte(`{"success":true,"result":{"uuid":"db-ok","name":"my-db"},"errors":[]}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))

	if err := preflightD1DatabaseBindings(context.Background(), dir, "tok", ""); err != nil {
		t.Fatalf("preflightD1DatabaseBindings: %v", err)
	}
}

func TestApplyBlocksBeforeWranglerWhenD1PreflightFails(t *testing.T) {
	tmp := t.TempDir()
	projectDir := seedD1WranglerConfigIn(t, filepath.Join(tmp, "project"), `wrong-id`, `my-db`)
	logPath := filepath.Join(tmp, "wrangler.log")
	installFakeWranglerAt(t, filepath.Join(projectDir, "node_modules", ".bin"), logPath, "https://demo.example.workers.dev")
	t.Setenv("PATH", t.TempDir())
	withCloudflareAPIServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"success":false,"result":null,"errors":[{"message":"database not found"}]}`))
	}))

	_, err := Apply(context.Background(), ApplyInput{
		ProjectDir:  projectDir,
		APIToken:    "tok",
		AccountID:   "acct",
		Env:         "prod",
		DryRun:      false,
		InjectedEnv: nil,
	})
	if err == nil {
		t.Fatal("expected preflight failure")
	}
	if raw, readErr := os.ReadFile(logPath); readErr == nil && strings.Contains(string(raw), "argv: deploy") {
		t.Fatalf("wrangler should not run after D1 preflight failure:\n%s", string(raw))
	}
}

func seedD1WranglerConfig(t *testing.T, databaseID, databaseName string) string {
	t.Helper()
	return seedD1WranglerConfigIn(t, t.TempDir(), databaseID, databaseName)
}

func seedD1WranglerConfigIn(t *testing.T, dir, databaseID, databaseName string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir project dir: %v", err)
	}
	body := `name = "web"
compatibility_date = "2024-09-23"

[[d1_databases]]
binding = "DB"
database_name = "` + databaseName + `"
database_id = "` + databaseID + `"
`
	if err := os.WriteFile(filepath.Join(dir, WranglerConfigFilename), []byte(body), 0o644); err != nil {
		t.Fatalf("write wrangler.toml: %v", err)
	}
	return dir
}

func withCloudflareAPIServer(t *testing.T, handler http.Handler) {
	t.Helper()
	prevBaseURL := cloudflareAPIBaseURL
	prevClient := cloudflareHTTPClient
	srv := httptest.NewServer(handler)
	t.Cleanup(func() {
		srv.Close()
		cloudflareAPIBaseURL = prevBaseURL
		cloudflareHTTPClient = prevClient
	})
	cloudflareAPIBaseURL = srv.URL + "/client/v4"
	cloudflareHTTPClient = srv.Client()
}
