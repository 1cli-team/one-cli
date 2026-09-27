package infisical

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	session "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/infisicalsession"
	"github.com/zalando/go-keyring"
)

func sharedTestSession(t *testing.T, site, user string) {
	t.Helper()
	raw, _ := json.Marshal(session.Session{Info: session.Info{SiteURL: site, UserID: user, OrganizationID: "org", ExpiresAt: time.Now().Add(time.Hour)}, Token: "test-token"})
	if err := keyring.Set("one-cli.infisical", "session", string(raw)); err != nil {
		t.Fatal(err)
	}
}

func TestDefaultSharedLocationConcurrentSetupAndPreservation(t *testing.T) {
	keyring.MockInit()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var mu sync.Mutex
	creates := 0
	project := RemoteProject{ID: "shared", Name: DefaultSharedProject, Type: "secret-manager", OrganizationID: "org", Environments: []RemoteEnvironment{{Slug: "dev"}}}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/workspace":
			json.NewEncoder(w).Encode(map[string]any{"workspaces": []RemoteProject{}})
		case "/api/v2/workspace":
			creates++
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			if body["projectName"] != DefaultSharedProject || body["type"] != "secret-manager" || r.Method != "POST" || r.Header.Get("Authorization") != "Bearer test-token" {
				t.Errorf("invalid project creation: %v", body)
			}
			json.NewEncoder(w).Encode(map[string]any{"project": project})
		case "/api/v1/workspace/shared":
			json.NewEncoder(w).Encode(map[string]any{"workspace": project})
		case "/api/v1/workspace/custom":
			json.NewEncoder(w).Encode(map[string]any{"workspace": RemoteProject{ID: "custom", Name: "Custom", Type: "secret-manager", OrganizationID: "org", Environments: []RemoteEnvironment{{Slug: "prod"}}}})
		default:
			t.Errorf("unexpected request: %s", r.URL)
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()
	sharedTestSession(t, upstream.URL, "user")
	ctx := context.Background()
	results := make(chan error, 4)
	for range 4 {
		go func() {
			location, err := EnsureDefaultGlobal(ctx)
			if err == nil && (location.ProjectID != "shared" || location.DefaultEnvironment != "dev") {
				t.Errorf("unexpected location: %+v", location)
			}
			results <- err
		}()
	}
	for range 4 {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	if creates != 1 {
		t.Fatalf("created %d projects", creates)
	}
	if _, err := BindGlobal(ctx, "custom", "prod"); err != nil {
		t.Fatal(err)
	}
	location, err := EnsureDefaultGlobal(ctx)
	if err != nil || location.ProjectID != "custom" || location.DefaultEnvironment != "prod" {
		t.Fatalf("existing location overwritten: %+v %v", location, err)
	}
	sharedTestSession(t, upstream.URL, "different-user")
	if _, err := EnsureDefaultGlobal(ctx); err == nil {
		t.Fatal("accepted another account's location")
	}
	location, _ = LoadGlobalLocation()
	if location.UserID != "user" {
		t.Fatal("overwrote location after account change")
	}
}

func TestSharedProjectFilteringAndDefaultReuse(t *testing.T) {
	for _, env := range []string{"dev", "prod"} {
		t.Run(env, func(t *testing.T) {
			keyring.MockInit()
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			project := RemoteProject{ID: "existing", Name: DefaultSharedProject, Type: "secret-manager", OrganizationID: "org", Environments: []RemoteEnvironment{{Slug: env}}}
			cert := RemoteProject{ID: "cert", Name: "Certificates", Type: "cert-manager", OrganizationID: "org"}
			foreign := project
			foreign.ID, foreign.OrganizationID = "foreign", "other-org"
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/api/v1/workspace":
					json.NewEncoder(w).Encode(map[string]any{"workspaces": []RemoteProject{cert, foreign, project}})
				case "/api/v1/workspace/existing":
					json.NewEncoder(w).Encode(map[string]any{"workspace": project})
				case "/api/v1/workspace/cert":
					json.NewEncoder(w).Encode(map[string]any{"workspace": cert})
				default:
					t.Errorf("must not create or read secrets: %s %s", r.Method, r.URL)
					http.NotFound(w, r)
				}
			}))
			defer upstream.Close()
			sharedTestSession(t, upstream.URL, "user")
			ctx := context.Background()
			projects, err := Projects(ctx)
			if err != nil || len(projects) != 1 || projects[0].ID != "existing" {
				t.Fatalf("wrong project filter: %+v %v", projects, err)
			}
			if _, err := BindGlobal(ctx, "cert", "dev"); err == nil {
				t.Fatal("bound certificate project")
			}
			location, err := EnsureDefaultGlobal(ctx)
			if env == "dev" {
				if err != nil || location.ProjectID != "existing" {
					t.Fatalf("did not reuse project: %+v %v", location, err)
				}
			} else {
				if err == nil {
					t.Fatal("silently switched default environment")
				}
				if location, _ = LoadGlobalLocation(); location != nil {
					t.Fatal("saved incomplete setup")
				}
			}
		})
	}
}

func TestCreateSharedProjectValidatesAndDoesNotBind(t *testing.T) {
	keyring.MockInit()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	creates := 0
	forbidden := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v2/workspace":
			creates++
			if forbidden {
				http.Error(w, "private-upstream-information", 403)
				return
			}
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			if body["projectName"] != "Team" || body["type"] != "secret-manager" {
				t.Errorf("invalid body %v", body)
			}
			w.Write([]byte(`{"project":{"id":"new","name":"Team"}}`))
		case "/api/v1/workspace/new":
			w.Write([]byte(`{"workspace":{"id":"new","name":"Team","type":"secret-manager","orgId":"org","environments":[{"slug":"dev"}]}}`))
		default:
			t.Errorf("unexpected request: %s", r.URL)
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()
	sharedTestSession(t, upstream.URL, "user")
	for _, name := range []string{"", "  ", "a\nb", strings.Repeat("a", 65)} {
		if _, err := CreateRemoteProject(context.Background(), name); err == nil {
			t.Errorf("accepted invalid name %q", name)
		}
	}
	if creates != 0 {
		t.Fatal("invalid name reached upstream")
	}
	project, err := CreateRemoteProject(context.Background(), " Team ")
	if err != nil || project.ID != "new" || len(project.Environments) != 1 {
		t.Fatalf("invalid created project: %+v %v", project, err)
	}
	if location, _ := LoadGlobalLocation(); location != nil {
		t.Fatal("creating a project changed saved location")
	}
	forbidden = true
	if _, err := CreateRemoteProject(context.Background(), "Team"); err == nil || strings.Contains(err.Error(), "private-upstream-information") {
		t.Fatalf("permission failure mishandled: %v", err)
	}
}
