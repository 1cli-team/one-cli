package cli_test

// E2E coverage of `one add` template mode.
//
// Ordinary `one add` renders a locally developable project and deliberately
// leaves deployment/container choices unset. The advanced
// --deploy-provider path remains available for automation.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSnapshot_E2E_Add_DefersDeploymentDefaults locks the central UX rule:
// adding a project does not choose a deployment target or render deploy
// artifacts, even when the technology stack has historical defaults.
func TestSnapshot_E2E_Add_DefersDeploymentDefaults(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)
	ws := bootstrapWorkspace(t, tmp, "ws")

	// go-api is the simplest template that exercises a non-default
	// toolchain (Go) without depending on node/pnpm at scaffold time.
	// Its technology-stack entry supports container/docker +
	// deploy/kustomize, but ordinary add must not select either one.
	stdout, stderr, code := runBinaryIn(t, ws, "add", "go-api", "--name", "user-api", "-y", "-o", "json")
	if code != 0 {
		t.Fatalf("add failed: exit %d\n  stdout: %s\n  stderr: %s", code, stdout, stderr)
	}
	got := mustParseJSON(t, stdout)
	if got["schema"] != "one-cli/add/v1" {
		t.Errorf("schema: want one-cli/add/v1, got %v", got["schema"])
	}
	if got["template_id"] != "go-api" {
		t.Errorf("template_id: want go-api, got %v", got["template_id"])
	}
	if got["toolchain"] != "go" {
		t.Errorf("toolchain: want go, got %v", got["toolchain"])
	}
	assertSnapshot(t, "add-go-api.json", got)

	// Subproject directory must exist with template artifacts.
	svcDir := filepath.Join(ws, "services", "user-api")
	for _, p := range []string{"go.mod", "Taskfile.yml", "cmd"} {
		full := filepath.Join(svcDir, p)
		if !fileExists(t, full) {
			t.Errorf("expected template file missing: %s", full)
		}
	}
	assertNoAgentDocs(t, ws)
	assertNoAgentDocs(t, svcDir)
	goModRaw, err := os.ReadFile(filepath.Join(svcDir, "go.mod"))
	if err != nil {
		t.Fatalf("read rendered go.mod: %v", err)
	}
	goMod := string(goModRaw)
	for _, want := range []string{
		"github.com/swaggo/files v1.0.1",
		"github.com/swaggo/gin-swagger v1.6.1",
	} {
		if !strings.Contains(goMod, want) {
			t.Errorf("rendered go.mod missing Swagger dependency %q:\n%s", want, goMod)
		}
	}
	routerRaw, err := os.ReadFile(filepath.Join(svcDir, "internal", "http", "router.go"))
	if err != nil {
		t.Fatalf("read rendered router.go: %v", err)
	}
	router := string(routerRaw)
	for _, want := range []string{
		`swaggerFiles "github.com/swaggo/files"`,
		`ginSwagger "github.com/swaggo/gin-swagger"`,
		`engine.GET("/api/docs/*any", ginSwagger.WrapHandler(`,
		`ginSwagger.URL("/api/openapi.yaml")`,
	} {
		if !strings.Contains(router, want) {
			t.Errorf("rendered router.go missing Swagger wiring %q:\n%s", want, router)
		}
	}
	appHandlerRaw, err := os.ReadFile(filepath.Join(svcDir, "internal", "http", "handlers", "app_handler.go"))
	if err != nil {
		t.Fatalf("read rendered app_handler.go: %v", err)
	}
	if !strings.Contains(string(appHandlerRaw), `c.Redirect(http.StatusTemporaryRedirect, "/api/docs/index.html")`) {
		t.Errorf("rendered app handler should redirect /api/docs to Swagger UI:\n%s", appHandlerRaw)
	}
	openAPIRaw, err := os.ReadFile(filepath.Join(svcDir, "api", "openapi.yaml"))
	if err != nil {
		t.Fatalf("read rendered openapi.yaml: %v", err)
	}
	if !strings.HasPrefix(string(openAPIRaw), "openapi: 3.0.3\n") {
		t.Errorf("rendered openapi.yaml must use Swagger UI-compatible OpenAPI 3.0.x, got:\n%s", openAPIRaw)
	}

	if fileExists(t, filepath.Join(svcDir, "Dockerfile")) {
		t.Error("ordinary add must not render a Dockerfile before deployment is chosen")
	}
	if fileExists(t, filepath.Join(ws, ".github", "workflows", "ci-services-user-api.yml")) {
		t.Error("ordinary add must not generate CI configuration")
	}
	for _, rel := range []string{
		"kustomize/base/user-api.yaml",
		"kustomize/base/kustomization.yaml",
		"kustomize/overlays/dev/kustomization.yaml",
		"kustomize/overlays/staging/kustomization.yaml",
		"kustomize/overlays/prod/kustomization.yaml",
	} {
		if fileExists(t, filepath.Join(ws, filepath.FromSlash(rel))) {
			t.Errorf("ordinary add unexpectedly rendered deployment artifact: %s", rel)
		}
	}

	// Manifest lists the project and its dev command, but deployment/container
	// remain absent until first deploy.
	mf := readManifest(t, ws)
	subs, _ := mf["projects"].([]any)
	if len(subs) != 1 {
		t.Fatalf("manifest: want 1 subproject, got %d", len(subs))
	}
	sub0 := subs[0].(map[string]any)
	domains, _ := sub0["domains"].(map[string]any)
	if _, ok := domains["container"]; ok {
		t.Errorf("subproject.domains.container should be absent, got %v", domains["container"])
	}
	if _, ok := domains["deploy"]; ok {
		t.Errorf("subproject.domains.deploy should be absent, got %v", domains["deploy"])
	}
	if sub0["buildVersion"] != "0.1.0" {
		t.Errorf("subproject.buildVersion: want 0.1.0, got %v", sub0["buildVersion"])
	}
}

func TestSnapshot_E2E_Add_S3BucketDefaultsToProjectID(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)
	ws := bootstrapWorkspace(t, tmp, "ws")

	_, stderr, code := runBinaryIn(t, ws, "add", "react-spa", "--name", "web", "--deploy-provider", "aws-s3", "-y", "-o", "json")
	if code != 0 {
		t.Fatalf("add failed: exit %d\n  stderr: %s", code, stderr)
	}

	mf := readManifest(t, ws)
	project, _ := mf["workspace"].(map[string]any)
	projectID, _ := project["id"].(string)
	if projectID == "" {
		t.Fatal("manifest project.id should be set")
	}
	subs, _ := mf["projects"].([]any)
	if len(subs) != 1 {
		t.Fatalf("manifest: want 1 subproject, got %d", len(subs))
	}
	sub0 := subs[0].(map[string]any)
	domains, _ := sub0["domains"].(map[string]any)
	deploy, _ := domains["deploy"].(map[string]any)
	// react-spa defaults to aws-s3 (registry.json) — any S3-compatible
	// kind would qualify for the bucket default behaviour exercised
	// below; pin to aws-s3 to keep the registry default explicit.
	if deploy == nil || deploy["kind"] != "aws-s3" {
		t.Fatalf("subproject.domains.deploy.kind: want aws-s3, got %v", domains["deploy"])
	}
	cfg, _ := deploy["config"].(map[string]any)
	if cfg == nil || cfg["bucket"] != projectID {
		t.Errorf("subproject.domains.deploy.config.bucket: want project.id %q, got %v", projectID, cfg["bucket"])
	}
}

func TestSnapshot_E2E_Add_CloudflareDeployWritesWranglerConfig(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)
	ws := bootstrapWorkspace(t, tmp, "ws")

	_, stderr, code := runBinaryIn(t, ws, "add", "astro-site", "--name", "web", "--deploy-provider", "cloudflare", "-y", "-o", "json")
	if code != 0 {
		t.Fatalf("add failed: exit %d\n  stderr: %s", code, stderr)
	}

	wranglerPath := filepath.Join(ws, "apps", "web", "wrangler.toml")
	raw, err := os.ReadFile(wranglerPath)
	if err != nil {
		t.Fatalf("expected wrangler.toml for deploy/cloudflare: %v", err)
	}
	body := string(raw)
	for _, want := range []string{
		`name = "web"`,
		"compatibility_date",
		"[assets]",
		`directory = "./dist"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("wrangler.toml missing %q:\n%s", want, body)
		}
	}

	mf := readManifest(t, ws)
	projects, _ := mf["projects"].([]any)
	if len(projects) != 1 {
		t.Fatalf("manifest: want 1 project, got %d", len(projects))
	}
	project := projects[0].(map[string]any)
	domains, _ := project["domains"].(map[string]any)
	deploy, _ := domains["deploy"].(map[string]any)
	if deploy == nil || deploy["kind"] != "cloudflare" {
		t.Fatalf("project.domains.deploy.kind: want cloudflare, got %v", domains["deploy"])
	}
	if _, ok := domains["container"]; ok {
		t.Fatalf("cloudflare deploy should not enable container/docker, got %v", domains["container"])
	}
	pkgRaw, err := os.ReadFile(filepath.Join(ws, "apps", "web", "package.json"))
	if err != nil {
		t.Fatalf("read generated package.json: %v", err)
	}
	pkg := mustParseJSON(t, string(pkgRaw))
	devDeps, _ := pkg["devDependencies"].(map[string]any)
	if devDeps["wrangler"] == nil {
		t.Fatalf("cloudflare deploy should add wrangler devDependency, got %v", devDeps)
	}
}

// TestSnapshot_E2E_Add_NoDefaultsTemplate locks the negative side of
// `defaults`: ts-library has no per-subproject defaults, so
// adding it under a default workspace must NOT produce a Dockerfile.
func TestSnapshot_E2E_Add_NoDefaultsTemplate(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)
	ws := bootstrapWorkspace(t, tmp, "ws")

	stdout, stderr, code := runBinaryIn(t, ws, "add", "ts-library", "--name", "shared", "-y", "-o", "json")
	if code != 0 {
		t.Fatalf("add failed: exit %d\n  stderr: %s", code, stderr)
	}
	_ = stdout

	libDir := filepath.Join(ws, "packages", "shared")
	if fileExists(t, filepath.Join(libDir, "Dockerfile")) {
		t.Error("Dockerfile produced for ts-library (no defaults expected)")
	}
}

// TestSnapshot_E2E_Add_GoLibTemplate verifies the go-lib template
// renders the golang-standards/project-layout starter (pkg/greeter,
// go.mod, Taskfile.yml, LICENSE) under packages/<name>/
// and — like ts-library — does NOT auto-enable container/docker or
// deploy/kustomize because it declares no per-subproject defaults.
func TestSnapshot_E2E_Add_GoLibTemplate(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)
	ws := bootstrapWorkspace(t, tmp, "ws")

	stdout, stderr, code := runBinaryIn(t, ws, "add", "go-lib", "--name", "mathx", "-y", "-o", "json")
	if code != 0 {
		t.Fatalf("add failed: exit %d\n  stdout: %s\n  stderr: %s", code, stdout, stderr)
	}
	got := mustParseJSON(t, stdout)
	if got["template_id"] != "go-lib" {
		t.Errorf("template_id: want go-lib, got %v", got["template_id"])
	}
	if got["toolchain"] != "go" {
		t.Errorf("toolchain: want go, got %v", got["toolchain"])
	}

	libDir := filepath.Join(ws, "packages", "mathx")
	for _, rel := range []string{
		"go.mod",
		"Taskfile.yml",
		"LICENSE",
		filepath.Join("pkg", "greeter", "greeter.go"),
		filepath.Join("pkg", "greeter", "greeter_test.go"),
	} {
		full := filepath.Join(libDir, rel)
		if !fileExists(t, full) {
			t.Errorf("expected go-lib artifact missing: %s", full)
		}
	}
	assertNoAgentDocs(t, ws)
	assertNoAgentDocs(t, libDir)

	// Dev-only go.mod must NOT leak into the rendered output.
	if raw, err := os.ReadFile(filepath.Join(libDir, "go.mod")); err == nil {
		if strings.Contains(string(raw), "template-go-lib") {
			t.Errorf("rendered go.mod still contains dev-only module name 'template-go-lib':\n%s", raw)
		}
		if !strings.Contains(string(raw), "mathx") {
			t.Errorf("rendered go.mod should reference project name 'mathx', got:\n%s", raw)
		}
	} else {
		t.Errorf("read rendered go.mod: %v", err)
	}

	// No defaults declared → no container/deploy artifacts.
	if fileExists(t, filepath.Join(libDir, "Dockerfile")) {
		t.Error("Dockerfile produced for go-lib (no defaults expected)")
	}
	if fileExists(t, filepath.Join(ws, "kustomize")) {
		t.Error("kustomize/ produced for go-lib (no defaults expected)")
	}

	// A library has no long-running local entrypoint. Declaring the Go API
	// fallback here would make the workspace overview claim it can start and
	// make `one dev` run a cmd/server directory the template does not contain.
	mf := readManifest(t, ws)
	projects, _ := mf["projects"].([]any)
	if len(projects) != 1 {
		t.Fatalf("manifest: want 1 project, got %d", len(projects))
	}
	project := projects[0].(map[string]any)
	domains, _ := project["domains"].(map[string]any)
	if dev, exists := domains["dev"]; exists {
		t.Fatalf("go-lib must not declare a runnable dev command, got %v", dev)
	}
}

func TestAddPreservesExistingAgentDocs(t *testing.T) {
	tmp := t.TempDir()
	isolateHome(t, tmp)
	ws := bootstrapWorkspace(t, tmp, "ws")
	files := map[string]string{
		"AGENTS.md":                  "<!-- one agents:index:start -->\nUser-owned rules\n<!-- one agents:index:end -->\n",
		"CLAUDE.md":                  "My Claude instructions\n",
		".one/agents/conventions.md": "My existing conventions\n",
	}
	for rel, content := range files {
		path := filepath.Join(ws, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	stdout, stderr, code := runBinaryIn(t, ws, "add", "go-api", "--name", "api", "--yes", "-o", "json")
	if code != 0 {
		t.Fatalf("add failed: exit=%d stderr=%s", code, stderr)
	}
	if _, ok := mustParseJSON(t, stdout)["ai_guides"]; ok {
		t.Fatal("add still reports automatic agent guide generation")
	}
	for rel, want := range files {
		got, err := os.ReadFile(filepath.Join(ws, filepath.FromSlash(rel)))
		if err != nil || string(got) != want {
			t.Errorf("existing %s changed: got=%q err=%v", rel, got, err)
		}
	}
	assertNoAgentDocs(t, filepath.Join(ws, "services", "api"))
}
