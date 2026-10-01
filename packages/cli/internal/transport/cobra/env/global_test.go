package envcmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/env/infisical"
	remote "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/environment"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/helpui"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	session "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/infisicalsession"
	"github.com/zalando/go-keyring"
)

func TestGlobalBindCreatesReusesAndPreservesStorage(t *testing.T) {
	for _, test := range []struct {
		name          string
		args          []string
		defaultExists bool
		savedCustom   bool
		forbidden     bool
		wantProject   string
		wantEnv       string
		wantCreates   int
		wantError     bool
	}{
		{name: "create default", wantProject: "shared", wantEnv: "dev", wantCreates: 1},
		{name: "reuse default", defaultExists: true, wantProject: "shared", wantEnv: "dev"},
		{name: "preserve custom binding", savedCustom: true, wantProject: "custom", wantEnv: "prod"},
		{name: "create with requested environment", args: []string{"--env", "prod"}, wantProject: "shared", wantEnv: "prod", wantCreates: 1},
		{name: "reuse with requested environment", defaultExists: true, args: []string{"--env", "prod"}, wantProject: "shared", wantEnv: "prod"},
		{name: "change saved environment", savedCustom: true, args: []string{"--env", "dev"}, wantProject: "custom", wantEnv: "dev"},
		{name: "explicit project", args: []string{"--project-id", "custom", "--env", "prod"}, wantProject: "custom", wantEnv: "prod"},
		{name: "invalid environment stays unbound", defaultExists: true, args: []string{"--env", "missing"}, wantError: true},
		{name: "invalid environment preserves binding", savedCustom: true, args: []string{"--env", "missing"}, wantProject: "custom", wantEnv: "prod", wantError: true},
		{name: "creation permission denied", forbidden: true, wantCreates: 1, wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			keyring.MockInit()
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			creates, lists := 0, 0
			shared := remote.RemoteProject{
				ID: "shared", Name: "shared-credentials", Type: "secret-manager", OrganizationID: "org",
				Environments: []infisical.RemoteEnvironment{{Slug: "dev"}, {Slug: "prod"}},
			}
			custom := shared
			custom.ID, custom.Name = "custom", "Custom"
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Header.Get("Authorization") != "Bearer test-token" {
					t.Error("request did not use the signed-in session")
				}
				switch r.URL.Path {
				case "/api/v1/workspace":
					lists++
					projects := []remote.RemoteProject{custom}
					if test.defaultExists {
						projects = append(projects, shared)
					}
					json.NewEncoder(w).Encode(map[string]any{"workspaces": projects})
				case "/api/v2/workspace":
					creates++
					if test.forbidden {
						http.Error(w, "private-upstream-information", http.StatusForbidden)
						return
					}
					var body map[string]string
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
						return
					}
					if r.Method != http.MethodPost || body["projectName"] != shared.Name || body["type"] != shared.Type {
						t.Errorf("unexpected project creation: %s %v", r.Method, body)
					}
					json.NewEncoder(w).Encode(map[string]any{"project": shared})
				case "/api/v1/workspace/shared":
					json.NewEncoder(w).Encode(map[string]any{"workspace": shared})
				case "/api/v1/workspace/custom":
					json.NewEncoder(w).Encode(map[string]any{"workspace": custom})
				default:
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
					http.NotFound(w, r)
				}
			}))
			defer upstream.Close()
			raw, err := json.Marshal(session.Session{
				Info:  session.Info{SiteURL: upstream.URL, UserID: "user", OrganizationID: "org", ExpiresAt: time.Now().Add(time.Hour)},
				Token: "test-token",
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := keyring.Set("one-cli.infisical", "session", string(raw)); err != nil {
				t.Fatal(err)
			}
			if test.savedCustom {
				if _, err := remote.BindGlobal(context.Background(), "custom", "prod"); err != nil {
					t.Fatal(err)
				}
			}
			for range 2 {
				cmd := Commands(Dependencies{})[0]
				cmd.SilenceUsage, cmd.SilenceErrors = true, true
				cmd.SetArgs(append([]string{"bind", "--global"}, test.args...))
				err := cmd.Execute()
				if (err != nil) != test.wantError {
					t.Fatalf("bind error = %v, wantError = %v", err, test.wantError)
				}
				if test.forbidden {
					var coded interface{ ErrorCode() string }
					if !errors.As(err, &coded) || coded.ErrorCode() != "INFISICAL_PROJECT_CREATE_FORBIDDEN" || strings.Contains(err.Error(), "private-upstream-information") {
						t.Fatalf("creation failure lost its error code or leaked upstream data: %v", err)
					}
				}
				location, err := remote.LoadGlobalLocation()
				if err != nil {
					t.Fatal(err)
				}
				if test.wantProject == "" {
					if location != nil {
						t.Fatalf("saved binding after failed setup: %+v", location)
					}
				} else if location == nil || location.ProjectID != test.wantProject || location.DefaultEnvironment != test.wantEnv || location.UserID != "user" || location.OrganizationID != "org" || location.SiteURL != upstream.URL {
					t.Fatalf("unexpected saved binding: %+v", location)
				}
				if test.wantError {
					break
				}
			}
			if creates != test.wantCreates {
				t.Fatalf("created %d projects, want %d", creates, test.wantCreates)
			}
			if (test.savedCustom || test.wantProject == "custom") && lists != 0 {
				t.Fatal("saved or explicit binding searched for the default project")
			}
		})
	}
}

func TestGlobalBindRequiresLogin(t *testing.T) {
	keyring.MockInit()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for _, args := range [][]string{{"bind", "--global"}} {
		cmd := Commands(Dependencies{})[0]
		cmd.SilenceUsage, cmd.SilenceErrors = true, true
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil {
			t.Fatalf("accepted binding without scope or login: %v", args)
		}
		if location, err := remote.LoadGlobalLocation(); err != nil || location != nil {
			t.Fatalf("created a binding without scope or login: %+v %v", location, err)
		}
	}
}

func TestGlobalBindHelpRefreshesInBothLanguages(t *testing.T) {
	t.Cleanup(func() { _ = i18n.Init(i18n.DefaultLocale) })
	cmd := Commands(Dependencies{})[0]
	cmd.SetHelpFunc(helpui.Render)
	bind, _, err := cmd.Find([]string{"bind"})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ locale, want, absent string }{
		{"en-US", "No project selection or ID is required", "无需选择项目或填写 ID"},
		{"zh-CN", "无需选择项目或填写 ID", "No project selection or ID is required"},
		{"en-US", "No project selection or ID is required", "无需选择项目或填写 ID"},
	} {
		if err := i18n.Init(test.locale); err != nil {
			t.Fatal(err)
		}
		i18n.RefreshTree(cmd)
		var out bytes.Buffer
		bind.SetOut(&out)
		if err := bind.Help(); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), test.want) || strings.Contains(out.String(), test.absent) || strings.Contains(out.String(), "env.bind.") {
			t.Fatalf("help did not refresh to %s: %s", test.locale, out.String())
		}
	}
}
