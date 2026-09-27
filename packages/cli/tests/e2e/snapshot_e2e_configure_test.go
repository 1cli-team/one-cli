package cli_test

// E2E coverage of configure profile creation and updates.

import (
	"strings"
	"testing"
)

func TestSnapshot_E2E_Configure_Env_Infisical(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)

	stdout, stderr, code := runBinary(t, "configure", "add", "env/infisical", "--profile", "work",
		"--site-url", "https://infisical.company.com",
		"--client-id", "cid-1", "--client-secret", "cs-1",
		"-o", "json")
	if code != 0 {
		t.Fatalf("configure add env/infisical: exit %d\n  stderr: %s", code, stderr)
	}
	got := mustParseJSON(t, stdout)
	if got["schema"] != "one-cli/configure-add/v1" {
		t.Errorf("schema: want one-cli/configure-add/v1, got %v", got["schema"])
	}
	if got["status"] != "completed" {
		t.Errorf("first add: want status=completed, got %v", got["status"])
	}
	if got["domain"] != "env" || got["backend"] != "infisical" || got["name"] != "work" {
		t.Errorf("payload mismatch: %s", pretty(got))
	}
	if got["default"] != true {
		t.Errorf("first add must be default (auto-default rule), got %v", got["default"])
	}
	assertSnapshot(t, "configure-add-env-infisical.json", got)

	stdout, _, code = runBinary(t, "configure", "list", "env/infisical", "-o", "json")
	if code != 0 {
		t.Fatalf("configure list env/infisical: exit %d", code)
	}
	if !strings.Contains(stdout, "\"name\":\"work\"") &&
		!strings.Contains(stdout, "\"name\": \"work\"") {
		t.Errorf("expected profile list to mention work; got %s", stdout)
	}
}

// Re-running profile add with the same name must update (not error).
// Locks the Upsert semantics adopted in v0.6 (replaces the old v0.5
// split between `setup` upsert and `<domain> profile add` insert-only).
func TestSnapshot_E2E_Configure_Idempotent(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)

	stdout, _, code := runBinary(t, "configure", "add", "env/infisical", "--profile", "work",
		"--site-url", "https://app.infisical.com",
		"--client-id", "cid-1", "--client-secret", "cs-1",
		"-o", "json")
	if code != 0 {
		t.Fatalf("first add: exit %d", code)
	}
	got1 := mustParseJSON(t, stdout)
	if got1["status"] != "completed" {
		t.Errorf("first run: want completed, got %v", got1["status"])
	}

	stdout, _, code = runBinary(t, "configure", "add", "env/infisical", "--profile", "work",
		"--site-url", "https://app.infisical.com",
		"--client-id", "cid-1", "--client-secret", "cs-2",
		"-o", "json")
	if code != 0 {
		t.Fatalf("second add: exit %d", code)
	}
	got2 := mustParseJSON(t, stdout)
	if got2["status"] != "updated" {
		t.Errorf("second run: want updated, got %v", got2["status"])
	}
	_ = tmp
}
