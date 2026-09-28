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

func TestSingleSessionPreservesProjectScopeAndRootPath(t *testing.T) {
	keyring.MockInit()
	raw, _ := json.Marshal(session.Session{Info: session.Info{UserID: "user", SiteURL: session.DefaultSiteURL, ExpiresAt: time.Now().Add(time.Hour)}, Token: "one-session"})
	if err := keyring.Set("one-cli.infisical", "session", string(raw)); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := workspace.WriteManifest(root, &workspace.Manifest{Version: workspace.ManifestVersion,
		Environments: &workspace.Environments{Names: []string{"dev"}, Default: "dev"},
		Env:          &workspace.EnvironmentConfig{ProjectID: "remote", RootPath: "/team", Keys: []string{"SHARED"}},
		Projects:     []workspace.ManifestProject{{Name: "web", RelativeDir: "apps/web"}, {Name: "api", RelativeDir: "services/api"}},
	}); err != nil {
		t.Fatal(err)
	}
	service := newTestService(t)
	cfg, credentials, err := service.resolveInfisical()
	if err != nil || credentials.AccessToken != "one-session" || cfg.SiteURL != session.DefaultSiteURL {
		t.Fatalf("session resolution failed: %v", err)
	}
	scope := execution.NewScope(context.Background(), root)
	active, err := execution.ResolveWorkspaceScope(scope)
	if err != nil {
		t.Fatal(err)
	}
	path, err := service.resolveInfisicalFolderPath(active, &infisical.WorkspaceConfig{SiteURL: session.DefaultSiteURL}, "")
	if err != nil || path != "/team" {
		t.Fatalf("root path = %q, %v", path, err)
	}
}
