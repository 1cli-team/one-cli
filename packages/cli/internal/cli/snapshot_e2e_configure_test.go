package cli_test

// E2E coverage of `one configure <verb> <domain>/<backend>` (verb-first
// tree, v0.7+) and `one skills install` (v0.6 replacement for the v0.5
// `one setup` command tree). Pin `--agent claude-code` on skills tests
// so the result doesn't depend on which agents happen to be installed
// on the host — that detection is the responsibility of internal/skill,
// not this command-level contract.
//
// Layout:
//   - Skills_Idempotent: `skills install --agent claude-code --yes`
//     twice produces identical envelopes (the contract carried over
//     from `one setup skills`, just under the new verb).
//   - Configure_Env_Infisical / Configure_Deploy_S3 /
//     Configure_Deploy_Kustomize / Configure_Container_Docker:
//     `add` writes a profile entry to ~/.config/one/config.json
//     (+ credentials.json for backends with secrets) and emits
//     one-cli/configure-add/v1.
//   - Configure_Idempotent: same name twice updates instead of errors
//     (Upsert semantics).

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSnapshot_E2E_Skills_Idempotent(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)

	stdout1, stderr, code := runBinary(t, "skills", "install", "--agent", "claude-code", "--yes", "-o", "json")
	if code != 0 {
		t.Fatalf("first skills install: exit %d\n  stderr: %s", code, stderr)
	}
	got1 := mustParseJSON(t, stdout1)
	if got1["schema"] != "one-cli/skills-install/v1" {
		t.Errorf("schema: want one-cli/skills-install/v1, got %v", got1["schema"])
	}
	if got1["status"] != "completed" {
		t.Errorf("status: want completed, got %v", got1["status"])
	}
	assertSnapshot(t, "skills-install-claude-code.json", got1)

	link := filepath.Join(tmp, ".claude", "skills", "one-cli")
	if !fileExists(t, link) {
		t.Errorf("expected skill link at %s", link)
	}

	stdout2, stderr, code := runBinary(t, "skills", "install", "--agent", "claude-code", "--yes", "-o", "json")
	if code != 0 {
		t.Fatalf("second skills install: exit %d\n  stderr: %s", code, stderr)
	}
	got2 := mustParseJSON(t, stdout2)
	if !reflect.DeepEqual(canonicalize(got1), canonicalize(got2)) {
		t.Errorf("skills install not idempotent: envelope changed between runs\n  run1: %s\n  run2: %s",
			pretty(canonicalize(got1)), pretty(canonicalize(got2)))
	}
}

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

func TestSnapshot_E2E_Configure_Deploy_AWSS3(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)

	stdout, stderr, code := runBinary(t, "configure", "add", "deploy/aws-s3", "--profile", "web-prod",
		"--region", "us-east-1",
		"--access-key-id", "ak", "--access-key-secret", "sk",
		"-o", "json")
	if code != 0 {
		t.Fatalf("configure add deploy/aws-s3: exit %d\n  stderr: %s", code, stderr)
	}
	got := mustParseJSON(t, stdout)
	if got["schema"] != "one-cli/configure-add/v1" {
		t.Errorf("schema: %v", got["schema"])
	}
	if got["domain"] != "deploy" || got["backend"] != "aws-s3" || got["name"] != "web-prod" {
		t.Errorf("payload: %s", pretty(got))
	}
	assertSnapshot(t, "configure-add-deploy-aws-s3.json", got)
	_ = tmp
}

func TestSnapshot_E2E_Configure_Deploy_AliyunOSS(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)

	stdout, stderr, code := runBinary(t, "configure", "add", "deploy/aliyun-oss", "--profile", "web-prod",
		"--endpoint", "https://oss-<region>.example.com",
		"--region", "cn-test",
		"--access-key-id", "ak", "--access-key-secret", "sk",
		"-o", "json")
	if code != 0 {
		t.Fatalf("configure add deploy/aliyun-oss: exit %d\n  stderr: %s", code, stderr)
	}
	got := mustParseJSON(t, stdout)
	if got["domain"] != "deploy" || got["backend"] != "aliyun-oss" || got["name"] != "web-prod" {
		t.Errorf("payload: %s", pretty(got))
	}
	assertSnapshot(t, "configure-add-deploy-aliyun-oss.json", got)
	_ = tmp
}

func TestSnapshot_E2E_Configure_Deploy_MinIO(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)

	stdout, stderr, code := runBinary(t, "configure", "add", "deploy/minio", "--profile", "lab",
		"--endpoint", "http://minio.test:9000",
		"--region", "us-east-1",
		"--force-path-style",
		"--access-key-id", "ak", "--access-key-secret", "sk",
		"-o", "json")
	if code != 0 {
		t.Fatalf("configure add deploy/minio: exit %d\n  stderr: %s", code, stderr)
	}
	got := mustParseJSON(t, stdout)
	if got["domain"] != "deploy" || got["backend"] != "minio" || got["name"] != "lab" {
		t.Errorf("payload: %s", pretty(got))
	}
	assertSnapshot(t, "configure-add-deploy-minio.json", got)
	_ = tmp
}

func TestSnapshot_E2E_Configure_Deploy_Kustomize(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)
	kubeconfig := writeTestKubeconfig(t, tmp, "prod", "prod")

	stdout, stderr, code := runBinary(t, "configure", "add", "deploy/kustomize", "--profile", "prod-k8s",
		"--kubeconfig", kubeconfig,
		"--kubeconfig-context", "prod",
		"-o", "json")
	if code != 0 {
		t.Fatalf("configure add deploy/kustomize: exit %d\n  stderr: %s", code, stderr)
	}
	got := mustParseJSON(t, stdout)
	if got["schema"] != "one-cli/configure-add/v1" {
		t.Errorf("schema: %v", got["schema"])
	}
	if got["domain"] != "deploy" || got["backend"] != "kustomize" {
		t.Errorf("payload: %s", pretty(got))
	}
	assertSnapshot(t, "configure-add-deploy-kustomize.json", got)
	_ = tmp
}

func TestSnapshot_E2E_Configure_Deploy_Vercel(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)

	stdout, stderr, code := runBinary(t, "configure", "add", "deploy/vercel", "--profile", "work",
		"--token", "vercel-tok-xyz",
		"--team", "acme",
		"-o", "json")
	if code != 0 {
		t.Fatalf("configure add deploy/vercel: exit %d\n  stderr: %s", code, stderr)
	}
	got := mustParseJSON(t, stdout)
	if got["schema"] != "one-cli/configure-add/v1" {
		t.Errorf("schema: %v", got["schema"])
	}
	if got["domain"] != "deploy" || got["backend"] != "vercel" || got["name"] != "work" {
		t.Errorf("payload: %s", pretty(got))
	}
	assertSnapshot(t, "configure-add-deploy-vercel.json", got)
	_ = tmp
}

func TestSnapshot_E2E_Configure_Deploy_Cloudflare(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)

	stdout, stderr, code := runBinary(t, "configure", "add", "deploy/cloudflare", "--profile", "work",
		"--token", "cf-tok-xyz",
		"--account-id", "acct-abc",
		"-o", "json")
	if code != 0 {
		t.Fatalf("configure add deploy/cloudflare: exit %d\n  stderr: %s", code, stderr)
	}
	got := mustParseJSON(t, stdout)
	if got["schema"] != "one-cli/configure-add/v1" {
		t.Errorf("schema: %v", got["schema"])
	}
	if got["domain"] != "deploy" || got["backend"] != "cloudflare" || got["name"] != "work" {
		t.Errorf("payload: %s", pretty(got))
	}
	assertSnapshot(t, "configure-add-deploy-cloudflare.json", got)
	_ = tmp
}

func TestSnapshot_E2E_Configure_Deploy_EdgeOne(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)

	stdout, stderr, code := runBinary(t, "configure", "add", "deploy/edgeone", "--profile", "work",
		"--token", "edgeone-token",
		"-o", "json")
	if code != 0 {
		t.Fatalf("configure add deploy/edgeone: exit %d\n  stderr: %s", code, stderr)
	}
	got := mustParseJSON(t, stdout)
	if got["schema"] != "one-cli/configure-add/v1" {
		t.Errorf("schema: %v", got["schema"])
	}
	if got["domain"] != "deploy" || got["backend"] != "edgeone" || got["name"] != "work" {
		t.Errorf("payload: %s", pretty(got))
	}
	assertSnapshot(t, "configure-add-deploy-edgeone.json", got)
	_ = tmp
}

func TestSnapshot_E2E_Configure_Container_Docker(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)

	stdout, stderr, code := runBinary(t, "configure", "add", "container/docker", "--profile", "acr-prod",
		"--registry", "registry.example.com",
		"--username", "u", "--password", "p",
		"-o", "json")
	if code != 0 {
		t.Fatalf("configure add container/docker: exit %d\n  stderr: %s", code, stderr)
	}
	got := mustParseJSON(t, stdout)
	if got["schema"] != "one-cli/configure-add/v1" {
		t.Errorf("schema: %v", got["schema"])
	}
	if got["domain"] != "container" || got["backend"] != "docker" || got["name"] != "acr-prod" {
		t.Errorf("payload: %s", pretty(got))
	}
	if got["default"] != true {
		t.Errorf("first add must be default, got %v", got["default"])
	}
	assertSnapshot(t, "configure-add-container-docker.json", got)

	stdout, _, code = runBinary(t, "configure", "list", "container/docker", "-o", "json")
	if code != 0 {
		t.Fatalf("configure list container/docker: exit %d", code)
	}
	if !strings.Contains(stdout, "acr-prod") {
		t.Errorf("expected profile list to mention acr-prod; got %s", stdout)
	}
	_ = tmp
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
