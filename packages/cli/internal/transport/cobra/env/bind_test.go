package envcmd

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	environmentmodule "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/environment"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	session "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/infisicalsession"
	"github.com/zalando/go-keyring"
)

func workspaceBindFixture(t *testing.T) (Dependencies, execution.Scope) {
	t.Helper()
	root := t.TempDir()
	if err := workspace.WriteManifest(root, &workspace.Manifest{
		Version: workspace.ManifestVersion, Workspace: &workspace.ManifestWorkspace{ID: "workspace", Name: "demo"},
	}); err != nil {
		t.Fatal(err)
	}
	service, err := environmentmodule.NewService(catalog.Builtin())
	if err != nil {
		t.Fatal(err)
	}
	return Dependencies{Service: service}, execution.NewScope(context.Background(), root)
}

func TestWorkspaceBindExplicitCreateAndExistingProject(t *testing.T) {
	for _, test := range []struct {
		name        string
		args        []string
		wantCreates int
	}{
		{"create", []string{"--create"}, 1},
		{"existing project", []string{"--project-id", "remote"}, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			keyring.MockInit()
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			creates := 0
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/api/v2/workspace":
					creates++
					var body map[string]string
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					if r.Method != http.MethodPost || body["projectName"] != "demo" {
						t.Errorf("unexpected creation: %s %v", r.Method, body)
					}
					w.Write([]byte(`{"project":{"id":"remote","name":"demo"}}`))
				case "/api/v1/workspace/remote":
					w.Write([]byte(`{"workspace":{"id":"remote","name":"Existing","type":"secret-manager","orgId":"org","environments":[{"slug":"dev"},{"slug":"qa"}]}}`))
				case "/api/v3/secrets/raw":
					if r.Method != http.MethodGet || r.URL.Query().Get("workspaceId") != "remote" {
						t.Errorf("unexpected verification: %s %s", r.Method, r.URL)
					}
					w.Write([]byte(`{"secrets":[]}`))
				default:
					t.Errorf("binding requested unexpected resource: %s %s", r.Method, r.URL)
					http.NotFound(w, r)
				}
			}))
			defer upstream.Close()
			raw, err := json.Marshal(session.Session{
				Info: session.Info{SiteURL: upstream.URL, UserID: "user", OrganizationID: "org", ExpiresAt: time.Now().Add(time.Hour)}, Token: "test-token",
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := keyring.Set("one-cli.infisical", "session", string(raw)); err != nil {
				t.Fatal(err)
			}
			deps, scope := workspaceBindFixture(t)
			for range 2 {
				cmd := Commands(deps)[0]
				cmd.SetContext(execution.WithScope(context.Background(), scope))
				cmd.SetArgs(append([]string{"bind"}, test.args...))
				if err := cmd.Execute(); err != nil {
					t.Fatal(err)
				}
			}
			manifest, err := workspace.ReadManifest(scope.WorkingDirectory())
			if err != nil || manifest.Env == nil || manifest.Env.ProjectID != "remote" || manifest.Env.SiteURL != upstream.URL {
				t.Fatalf("binding was not saved: %+v %v", manifest, err)
			}
			if test.wantCreates == 0 && strings.Join(manifest.Env.Environments, ",") != "dev,qa" {
				t.Fatalf("did not use remote environments: %v", manifest.Env.Environments)
			}
			if creates != test.wantCreates {
				t.Fatalf("created %d projects, want %d", creates, test.wantCreates)
			}
			if location, err := environmentmodule.LoadGlobalLocation(); err != nil || location != nil {
				t.Fatalf("workspace binding changed global credentials: %+v %v", location, err)
			}
		})
	}
}

func TestUnboundSetFailsBeforePromptInBothLanguages(t *testing.T) {
	keyring.MockInit()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Cleanup(func() { _ = i18n.Init(i18n.DefaultLocale) })
	deps, scope := workspaceBindFixture(t)
	before, err := os.ReadFile(workspace.ManifestPath(scope.WorkingDirectory()))
	if err != nil {
		t.Fatal(err)
	}
	for _, locale := range []string{"zh-CN", "en-US"} {
		if err := i18n.Init(locale); err != nil {
			t.Fatal(err)
		}
		for _, args := range [][]string{{"set", "API_TOKEN"}, {"set", "API_TOKEN=private-value"}} {
			cmd := Commands(deps)[0]
			cmd.SilenceErrors, cmd.SilenceUsage = true, true
			cmd.SetContext(execution.WithScope(context.Background(), scope))
			cmd.SetArgs(args)
			err := cmd.Execute()
			var coded interface{ ErrorCode() string }
			if !errors.As(err, &coded) || coded.ErrorCode() != "INFISICAL_NOT_CONFIGURED" || !strings.Contains(err.Error(), "one env bind") || strings.Contains(err.Error(), "private-value") {
				t.Fatalf("unexpected unbound %s error: %v", locale, err)
			}
		}
	}
	after, err := os.ReadFile(workspace.ManifestPath(scope.WorkingDirectory()))
	if err != nil || string(before) != string(after) {
		t.Fatalf("unbound set changed the manifest: %v", err)
	}
}

func TestWorkspaceBindRejectsAmbiguousFlagsBeforeRemoteAccess(t *testing.T) {
	for _, args := range [][]string{
		{"bind", "--create", "--project-id", "remote"},
		{"bind", "--create", "--env", "prod"},
		{"bind", "--create", "--path", "/"},
	} {
		cmd := Commands(Dependencies{})[0]
		cmd.SilenceErrors, cmd.SilenceUsage = true, true
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil {
			t.Fatalf("accepted ambiguous binding flags: %v", args)
		}
	}
}
