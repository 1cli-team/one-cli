package infisical_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/application/execution"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/environment"
	cliErrors "github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/plugins"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/plugins/infisical"
	session "github.com/torchstellar-team/one-cli/packages/one-core/pkg/plugins/infisical/session"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/ports/secrets"
)

func writeFixture(t *testing.T, path string, value []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, value, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestPluginRemoteOperationsAndInvocationSnapshot(t *testing.T) {
	t.Setenv("ONE_HOME", t.TempDir())
	var snapshots atomic.Int32
	var updates atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("Authorization") != "Bearer fixture-token" {
			t.Error("missing session authentication")
		}
		if r.URL.Query().Get("environment") != "dev" && r.Method == http.MethodGet {
			t.Error("wrong environment")
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v3/secrets/raw":
			if r.URL.Query().Get("recursive") == "true" {
				version := snapshots.Add(1)
				_ = json.NewEncoder(w).Encode(map[string]any{"secrets": []map[string]any{
					{"secretKey": "SHARED", "secretValue": "root", "secretPath": "/"},
					{"secretKey": "OVERRIDE", "secretValue": "parent", "secretPath": "/services"},
					{"secretKey": "OVERRIDE", "secretValue": "api", "secretPath": "/services/api"},
					{"secretKey": "VERSION", "secretValue": string(rune('0' + version)), "secretPath": "/services/api"},
					{"secretKey": "WEB_ONLY", "secretValue": "web", "secretPath": "/apps/web"},
				}})
			} else {
				if r.URL.Query().Get("secretPath") != "/services/api" {
					t.Error("wrong project folder")
				}
				_, _ = w.Write([]byte(`{"secrets":[{"secretKey":"EXAMPLE","secretValue":"private-value","secretPath":"/services/api"}]}`))
			}
		case r.Method == http.MethodGet && r.URL.Path == "/api/v3/secrets/raw/EXAMPLE":
			_, _ = w.Write([]byte(`{"secret":{"secretKey":"EXAMPLE","secretValue":"private-value"}}`))
		case r.Method == http.MethodPatch && r.URL.Path == "/api/v3/secrets/raw/EXAMPLE":
			updates.Add(1)
			_, _ = w.Write([]byte(`{"secret":{"secretKey":"EXAMPLE","secretValue":"updated"}}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v3/secrets/raw/EXAMPLE":
			_, _ = w.Write([]byte(`{"secret":{"secretKey":"EXAMPLE"}}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected", 500)
		}
	}))
	defer server.Close()
	root := t.TempDir()
	writeFixture(t, filepath.Join(root, "one.manifest.toml"), []byte("version = 2\n[workspace]\nid = 'test'\nname = 'Test'\n[env.infisical]\nsiteUrl = '"+server.URL+"'\nprojectId = 'fixture-project'\nenvironments = ['dev']\n[projects.api]\npath = 'services/api'\ntoolchain = 'go'\n[projects.web]\npath = 'apps/web'\ntoolchain = 'go'\n"))
	path, err := session.ConfigPath("session.json")
	if err != nil {
		t.Fatal(err)
	}
	stored, err := json.Marshal(session.Session{Info: session.Info{SiteURL: server.URL, ExpiresAt: time.Now().Add(time.Hour)}, Token: "fixture-token"})
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, path, stored)
	registry, err := plugins.New(infisical.New())
	if err != nil {
		t.Fatal(err)
	}
	scope := execution.NewScope(context.Background(), filepath.Join(root, "services", "api"))
	listing, err := registry.Environments.List(context.Background(), environment.ListInput{Scope: scope})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(listing.Keys, []string{"EXAMPLE"}) {
		t.Fatalf("keys = %v", listing.Keys)
	}
	encoded, _ := json.Marshal(listing)
	if strings.Contains(string(encoded), "private-value") {
		t.Fatal("invalid listing")
	}
	value, err := registry.Environments.Get(context.Background(), environment.GetInput{Scope: scope, Key: "EXAMPLE"})
	if err != nil || value.Value != "private-value" {
		t.Fatalf("get = %v, %v", value, err)
	}
	plan, err := registry.Environments.PlanSet(environment.PlanSetInput{Scope: scope})
	if err != nil {
		t.Fatal(err)
	}
	set, err := registry.Environments.Set(context.Background(), environment.SetInput{Plan: plan, Key: "EXAMPLE", Value: "updated", Overwrite: true})
	if err != nil || set.Action != "updated" || updates.Load() != 1 {
		t.Fatalf("set = %v, %v", set, err)
	}
	deleted, err := registry.Environments.Delete(context.Background(), environment.DeleteInput{Scope: scope, Key: "EXAMPLE"})
	if err != nil || deleted.Status != "deleted" {
		t.Fatalf("delete = %v, %v", deleted, err)
	}
	loader := registry.Loaders.Find("infisical")
	values, err := secrets.LoadProjects(context.Background(), loader, root, []string{"services/api", "apps/web"}, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if values["services/api"]["OVERRIDE"] != "api" || values["services/api"]["SHARED"] != "root" || values["services/api"]["WEB_ONLY"] != "" || values["apps/web"]["OVERRIDE"] != "" {
		t.Fatalf("incorrect inheritance: %v", values)
	}
	if snapshots.Load() != 1 {
		t.Fatal("projects did not share an invocation snapshot")
	}
	fresh, err := loader.Load(context.Background(), root, "services/api", "dev")
	if err != nil || fresh["VERSION"] == values["services/api"]["VERSION"] {
		t.Fatalf("snapshot was reused: %v, %v", fresh, err)
	}
	if err := session.Logout(); err != nil {
		t.Fatal(err)
	}
	_, err = loader.Load(context.Background(), root, "services/api", "dev")
	var coded interface{ ErrorCode() string }
	if !errors.As(err, &coded) || coded.ErrorCode() != string(cliErrors.INFISICAL_AUTH_MISSING) {
		t.Fatalf("missing auth = %v", err)
	}
}

func TestRegistryWithoutInfisical(t *testing.T) {
	registry, err := plugins.New()
	if err != nil {
		t.Fatal(err)
	}
	if len(registry.Catalog.All()) != 0 || registry.Loaders.Find("infisical") != nil || registry.Global != nil {
		t.Fatal("plugin leaked into empty registry")
	}
	if _, err := plugins.New(infisical.New(), infisical.New()); err == nil {
		t.Fatal("duplicate plugin accepted")
	}
}
