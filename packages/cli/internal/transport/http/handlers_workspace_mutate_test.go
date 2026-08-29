package serve

// Locks the HTTP contract for the application/workspace mutation service:
// each PUT returns a fresh Overview and preserves the existing status/error
// envelopes. Same isolated-tmp pattern as handlers_workspace_test.go.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
)

// seedWorkspace writes a minimal current manifest with one app project that has
// no env/deploy/container configured — i.e. the Overview flags it.
func seedWorkspace(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	m := &workspace.Manifest{
		Version:   workspace.ManifestVersion,
		Workspace: &workspace.ManifestWorkspace{ID: "demo", Name: "demo"},
		Projects: []workspace.ManifestProject{
			{Name: "web", RelativeDir: "apps/web", TemplateID: "react-spa", Toolchain: "node"},
		},
	}
	if err := workspace.WriteManifest(tmp, m); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}
	return tmp
}

func TestPutWorkspaceEnv_SetsKind(t *testing.T) {
	root := seedWorkspace(t)
	srv, _ := newOverviewServer(t, root)

	body := strings.NewReader(`{"kind":"dotenv"}`)
	res, raw := authedRequest(t, srv, http.MethodPut, "/api/workspace/domains/env", body)
	if res.StatusCode != 200 {
		t.Fatalf("status = %d; body = %s", res.StatusCode, raw)
	}
	// Response is the fresh Overview — the workspace env issue should be gone.
	var ov workspace.Overview
	if err := json.Unmarshal(raw, &ov); err != nil {
		t.Fatalf("decode: %v (%s)", err, raw)
	}
	for _, iss := range ov.Issues {
		if iss.Domain == workspace.IssueDomainEnv {
			t.Errorf("env issue still present after mutate: %+v", ov.Issues)
		}
	}
	// On-disk verification.
	m, err := workspace.ReadManifest(root)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	if workspace.EnvBackend(m) != workspace.EnvBackendDotenv {
		t.Errorf("manifest env kind = %q; want dotenv", workspace.EnvBackend(m))
	}
}

func TestPutWorkspaceEnv_RejectsUnknownKind(t *testing.T) {
	root := seedWorkspace(t)
	srv, _ := newOverviewServer(t, root)
	body := strings.NewReader(`{"kind":"vault"}`)
	res, raw := authedRequest(t, srv, http.MethodPut, "/api/workspace/domains/env", body)
	if res.StatusCode != 400 {
		t.Fatalf("status = %d; want 400; body = %s", res.StatusCode, raw)
	}
}

func TestPutProjectDeploy_SetsKind(t *testing.T) {
	root := seedWorkspace(t)
	srv, _ := newOverviewServer(t, root)

	body := strings.NewReader(`{"kind":"vercel"}`)
	res, raw := authedRequest(t, srv, http.MethodPut, "/api/workspace/projects/web/deploy", body)
	if res.StatusCode != 200 {
		t.Fatalf("status = %d; body = %s", res.StatusCode, raw)
	}
	var ov workspace.Overview
	if err := json.Unmarshal(raw, &ov); err != nil {
		t.Fatalf("decode: %v (%s)", err, raw)
	}
	for _, iss := range ov.Projects[0].Issues {
		if iss.Domain == workspace.IssueDomainDeploy && iss.Reason == workspace.IssueReasonBackend {
			t.Errorf("deploy backend issue still present after mutate: %+v", ov.Projects[0].Issues)
		}
	}
	m, _ := workspace.ReadManifest(root)
	if got := workspace.DeployForProject(m, "web").Backend; got != workspace.DeployBackendVercel {
		t.Errorf("manifest deploy kind = %q; want vercel", got)
	}
}

func TestPutProjectDeploy_RejectsIncompatibleKindWithoutChangingManifest(t *testing.T) {
	root := seedWorkspace(t)
	m, err := workspace.ReadManifest(root)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	m.Projects[0].Name = "api"
	m.Projects[0].RelativeDir = "services/api"
	m.Projects[0].TemplateID = "go-api"
	if err := workspace.WriteManifest(root, m); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}
	manifestPath := filepath.Join(root, "one.manifest.json")
	before, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest before request: %v", err)
	}

	srv, _ := newOverviewServer(t, root)
	body := strings.NewReader(`{"kind":"vercel"}`)
	res, raw := authedRequest(t, srv, http.MethodPut, "/api/workspace/projects/api/deploy", body)
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d; want 400; body = %s", res.StatusCode, raw)
	}
	var envelope struct {
		Schema string `json:"schema"`
		Error  struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("decode error envelope: %v (%s)", err, raw)
	}
	if envelope.Schema != "one-cli/error/v1" || envelope.Error.Code != "SERVE_PAYLOAD_INVALID" {
		t.Fatalf("unexpected error contract: %+v", envelope)
	}
	after, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest after request: %v", err)
	}
	if !bytes.Equal(after, before) {
		t.Fatal("incompatible deploy request changed one.manifest.json")
	}
}

func TestPutProjectDeploy_UnknownProject_404(t *testing.T) {
	root := seedWorkspace(t)
	srv, _ := newOverviewServer(t, root)
	body := strings.NewReader(`{"kind":"vercel"}`)
	res, raw := authedRequest(t, srv, http.MethodPut, "/api/workspace/projects/ghost/deploy", body)
	if res.StatusCode != 404 {
		t.Fatalf("status = %d; want 404; body = %s", res.StatusCode, raw)
	}
}

func TestPutProjectContainer_EnablesAndSetsKind(t *testing.T) {
	root := seedWorkspace(t)
	srv, _ := newOverviewServer(t, root)

	body := strings.NewReader(`{"kind":"ghcr","image":"web:latest"}`)
	res, raw := authedRequest(t, srv, http.MethodPut, "/api/workspace/projects/web/container", body)
	if res.StatusCode != 200 {
		t.Fatalf("status = %d; body = %s", res.StatusCode, raw)
	}
	var ov workspace.Overview
	if err := json.Unmarshal(raw, &ov); err != nil {
		t.Fatalf("decode: %v (%s)", err, raw)
	}
	for _, iss := range ov.Projects[0].Issues {
		if iss.Domain == workspace.IssueDomainContainer && iss.Reason == workspace.IssueReasonBackend {
			t.Errorf("container backend issue still present after mutate: %+v", ov.Projects[0].Issues)
		}
	}
	m, _ := workspace.ReadManifest(root)
	enabled, image := workspace.ContainerForProject(m, "web")
	if !enabled {
		t.Errorf("container not enabled after mutate")
	}
	if image != "web:latest" {
		t.Errorf("image = %q; want web:latest", image)
	}
	if got := workspace.ContainerKindForProject(m, "web"); got != "ghcr" {
		t.Errorf("container kind = %q; want ghcr", got)
	}
}

// Container kind is optional — an empty kind is allowed (inherits the
// workspace default / "docker"). The presence of the block is what
// enables container builds.
func TestPutProjectContainer_EmptyKindAllowed(t *testing.T) {
	root := seedWorkspace(t)
	srv, _ := newOverviewServer(t, root)
	body := strings.NewReader(`{}`)
	res, raw := authedRequest(t, srv, http.MethodPut, "/api/workspace/projects/web/container", body)
	if res.StatusCode != 200 {
		t.Fatalf("status = %d; body = %s", res.StatusCode, raw)
	}
	m, _ := workspace.ReadManifest(root)
	if enabled, _ := workspace.ContainerForProject(m, "web"); !enabled {
		t.Errorf("container not enabled with empty-kind body")
	}
}

func TestPutWorkspaceEnv_NoWorkspace_409(t *testing.T) {
	srv, _ := newOverviewServer(t, "")
	body := strings.NewReader(`{"kind":"dotenv"}`)
	res, raw := authedRequest(t, srv, http.MethodPut, "/api/workspace/domains/env", body)
	if res.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d; want 409; body = %s", res.StatusCode, raw)
	}
}

// Mutations are gated by the same Origin check as the configure routes.
func TestPutWorkspaceEnv_RejectsCrossOrigin(t *testing.T) {
	root := seedWorkspace(t)
	srv, _ := newOverviewServer(t, root)
	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/workspace/domains/env",
		strings.NewReader(`{"kind":"dotenv"}`))
	req.AddCookie(&http.Cookie{Name: tokenCookie, Value: testToken})
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://attacker.example.com")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d; want 403", res.StatusCode)
	}
}
