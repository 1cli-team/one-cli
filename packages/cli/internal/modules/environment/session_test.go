package environment

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/env/infisical"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	session "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/infisicalsession"
	"github.com/zalando/go-keyring"
)

func TestSingleSessionPullPreservesProjectScopeAndRootPath(t *testing.T) {
	keyring.MockInit()
	raw, _ := json.Marshal(session.Session{Info: session.Info{UserID: "user", SiteURL: session.DefaultSiteURL, ExpiresAt: time.Now().Add(time.Hour)}, Token: "one-session"})
	if err := keyring.Set("one-cli.infisical", "session", string(raw)); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := workspace.WriteManifest(root, &workspace.Manifest{Version: workspace.ManifestVersion,
		Environments: &workspace.Environments{Names: []string{"dev"}, Default: "dev"},
		Domains:      &workspace.WorkspaceDomains{Env: &workspace.BackendRef{Kind: "infisical", Config: json.RawMessage(`{"projectId":"remote","rootPath":"/team","keys":["SHARED"]}`)}},
		Projects:     []workspace.ManifestProject{{Name: "web", RelativeDir: "apps/web"}, {Name: "api", RelativeDir: "services/api"}},
	}); err != nil {
		t.Fatal(err)
	}
	service := newTestService(t)
	seen := map[string]bool{}
	service.pullInfisical = func(_ context.Context, _ string, in infisical.PullInput) (*infisical.PullResult, error) {
		if in.Creds.AccessToken != "one-session" || in.Cfg.SiteURL != session.DefaultSiteURL || in.Env != "dev" {
			t.Fatalf("incorrect session/scope: %#v", in)
		}
		seen[in.Project] = true
		return &infisical.PullResult{Env: in.Env, WrittenCount: 1}, nil
	}
	scope := execution.NewScope(context.Background(), root)
	result, err := service.Pull(context.Background(), PullInput{Scope: scope, Environment: "dev"})
	if err != nil {
		t.Fatal(err)
	}
	if result.WrittenCount != 3 || !seen["/"] || !seen["web"] || !seen["api"] {
		t.Fatalf("pull scopes: %v result: %#v", seen, result)
	}
	active, err := execution.ResolveWorkspaceScope(scope)
	if err != nil {
		t.Fatal(err)
	}
	path, err := service.resolveInfisicalFolderPath(active, &infisical.WorkspaceConfig{SiteURL: session.DefaultSiteURL}, "")
	if err != nil || path != "/team" {
		t.Fatalf("root path = %q, %v", path, err)
	}
}
