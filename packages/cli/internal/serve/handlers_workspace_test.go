package serve

// Locks the HTTP wire contract for /workspace/overview. Same isolated-tmp
// pattern as handlers_configure_test.go; just swaps which directory the
// handler reads from instead of XDG_CONFIG_HOME.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

// newOverviewServer builds a mux with WorkspaceRoot pinned to root. Mirrors
// newTestServer's two-step "patch hosts after addr is known" dance.
func newOverviewServer(t *testing.T, root string) (*httptest.Server, string) {
	t.Helper()
	withIsolatedConfig(t)
	mux := BuildMux(MuxOpts{
		Token:         testToken,
		UIDisabled:    true,
		WorkspaceRoot: root,
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	addr := strings.TrimPrefix(srv.URL, "http://")
	srv.Config.Handler = BuildMux(MuxOpts{
		Token:         testToken,
		UIDisabled:    true,
		ExpectedHosts: map[string]struct{}{addr: {}},
		SelfOrigin:    srv.URL,
		WorkspaceRoot: root,
	})
	return srv, srv.URL
}

func TestOverview_NoWorkspace_ReturnsPresentFalse(t *testing.T) {
	srv, _ := newOverviewServer(t, "")
	res, raw := authedRequest(t, srv, http.MethodGet, "/api/workspace/overview", nil)
	if res.StatusCode != 200 {
		t.Fatalf("status = %d; body = %s", res.StatusCode, raw)
	}
	var got workspace.Overview
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode: %v (%s)", err, raw)
	}
	if got.Schema != workspace.OverviewSchema {
		t.Errorf("schema = %q", got.Schema)
	}
	if got.Present {
		t.Errorf("present = true; want false")
	}
}

func TestOverview_PopulatedWorkspace(t *testing.T) {
	tmp := t.TempDir()
	deployCfg, _ := json.Marshal(map[string]any{"projectId": "x"})
	m := &workspace.Manifest{
		Version:      workspace.ManifestVersion,
		Workspace:    &workspace.ManifestWorkspace{ID: "demo", Name: "demo"},
		Environments: &workspace.Environments{Names: []string{"dev"}, Default: "dev"},
		Domains: &workspace.WorkspaceDomains{
			Env: &workspace.BackendRef{Kind: workspace.EnvBackendDotenv},
		},
		Projects: []workspace.ManifestProject{
			{
				Name: "web", RelativeDir: "apps/web", TemplateID: "react-spa", Toolchain: "node",
				Domains: &workspace.ProjectDomains{
					Deploy: &workspace.ProjectDeployBackend{Kind: workspace.DeployBackendVercel, Config: deployCfg},
					Dev:    &workspace.ProjectDevOverride{Command: "pnpm dev"},
				},
			},
		},
	}
	if err := workspace.WriteManifest(tmp, m); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}

	srv, _ := newOverviewServer(t, tmp)
	res, raw := authedRequest(t, srv, http.MethodGet, "/api/workspace/overview", nil)
	if res.StatusCode != 200 {
		t.Fatalf("status = %d; body = %s", res.StatusCode, raw)
	}
	var got workspace.Overview
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode: %v (%s)", err, raw)
	}
	if !got.Present {
		t.Fatalf("present = false; want true")
	}
	if len(got.Projects) != 1 || got.Projects[0].Name != "web" {
		t.Errorf("projects = %+v", got.Projects)
	}
	// Vercel deploy does not need container; the only remaining project
	// issue is the missing Vercel credential profile.
	if len(got.Projects[0].Issues) != 1 || got.Projects[0].Issues[0].Domain != workspace.IssueDomainDeploy || got.Projects[0].Issues[0].Reason != workspace.IssueReasonProfile {
		t.Errorf("project issues = %+v; want one deploy profile issue", got.Projects[0].Issues)
	}
}

func TestOverview_BadManifest_500WithErrorEnvelope(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, workspace.ManifestFilename), []byte("{not json"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	srv, _ := newOverviewServer(t, tmp)
	res, raw := authedRequest(t, srv, http.MethodGet, "/api/workspace/overview", nil)
	if res.StatusCode != 500 {
		t.Fatalf("status = %d; body = %s", res.StatusCode, raw)
	}
	var env map[string]any
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("decode envelope: %v (%s)", err, raw)
	}
	if env["schema"] != "one-cli/error/v1" {
		t.Errorf("schema = %v", env["schema"])
	}
}
