package environment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/env/infisical"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	session "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/infisicalsession"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/output"
	"github.com/zalando/go-keyring"
)

func unboundScope(t *testing.T) execution.Scope {
	t.Helper()
	root := filepath.Join(t.TempDir(), "demo")
	if err := os.Mkdir(root, 0755); err != nil {
		t.Fatal(err)
	}
	if err := workspace.WriteManifest(root, &workspace.Manifest{Version: workspace.ManifestVersion, Workspace: &workspace.ManifestWorkspace{ID: workspace.GenerateProjectID("demo"), Name: "demo"}}); err != nil {
		t.Fatal(err)
	}
	return execution.NewScope(context.Background(), root)
}

func TestReadsAndDeletesNeverInitializeOrModifyAnUnboundWorkspace(t *testing.T) {
	service := newTestService(t)
	service.initInfisical = func(context.Context, string, infisical.InitInput) (*infisical.InitResult, error) {
		t.Fatal("read/delete initialized storage")
		return nil, nil
	}
	scope := unboundScope(t)
	before, _ := os.ReadFile(workspace.ManifestPath(scope.WorkingDirectory()))
	for _, operation := range []func() error{
		func() error { _, err := service.PlanSet(PlanSetInput{Scope: scope}); return err },
		func() error { _, err := service.List(context.Background(), ListInput{Scope: scope}); return err },
		func() error {
			_, err := service.Get(context.Background(), GetInput{Scope: scope, Key: "TOKEN"})
			return err
		},
		func() error {
			_, err := service.Delete(context.Background(), DeleteInput{Scope: scope, Key: "TOKEN"})
			return err
		},
	} {
		err := operation()
		var coded *output.Error
		if !errors.As(err, &coded) || coded.Code != "INFISICAL_NOT_CONFIGURED" {
			t.Fatalf("error=%v", err)
		}
	}
	after, _ := os.ReadFile(workspace.ManifestPath(scope.WorkingDirectory()))
	if string(before) != string(after) {
		t.Fatal("query changed workspace")
	}
}

func mockSession(t *testing.T, site string) {
	t.Helper()
	keyring.MockInit()
	raw, _ := json.Marshal(session.Session{Info: session.Info{UserID: "test", SiteURL: site, ExpiresAt: time.Now().Add(time.Hour)}, Token: "test-token"})
	if err := keyring.Set("one-cli.infisical", "session", string(raw)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = keyring.Delete("one-cli.infisical", "session") })
}

func TestWorkspaceBindingRejectsStaleRevisionBeforeRemoteWork(t *testing.T) {
	service := newTestService(t)
	scope := unboundScope(t)
	service.initInfisical = func(context.Context, string, infisical.InitInput) (*infisical.InitResult, error) {
		t.Fatal("stale binding contacted Infisical")
		return nil, nil
	}
	_, err := service.BindWorkspace(context.Background(), BindWorkspaceInput{Scope: scope, Create: true, Revision: "stale"})
	var coded *output.Error
	if !errors.As(err, &coded) || coded.Code != "SERVE_MANIFEST_CONFLICT" {
		t.Fatalf("expected revision conflict, got %v", err)
	}
}

func TestWorkspaceBindingPreservesExternalEditDuringCreation(t *testing.T) {
	service := newTestService(t)
	scope := unboundScope(t)
	root := scope.WorkingDirectory()
	_, revision, err := workspace.ReadManifestSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	creates := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v2/workspace":
			creates++
			manifest, err := workspace.ReadManifest(root)
			if err != nil {
				t.Error(err)
			}
			manifest.Workspace.Name = "edited-during-creation"
			if err := workspace.WriteManifest(root, manifest); err != nil {
				t.Error(err)
			}
			fmt.Fprint(w, `{"project":{"id":"created","name":"demo"}}`)
		case "/api/v1/workspace/created":
			fmt.Fprint(w, `{"workspace":{"id":"created","name":"demo","type":"secret-manager","environments":[{"slug":"dev"}]}}`)
		case "/api/v3/secrets/raw":
			fmt.Fprint(w, `{"secrets":[]}`)
		default:
			t.Errorf("unexpected request: %s", r.URL)
		}
	}))
	defer upstream.Close()
	mockSession(t, upstream.URL)
	_, err = service.BindWorkspace(context.Background(), BindWorkspaceInput{Scope: scope, Create: true, Revision: revision})
	var coded *output.Error
	if !errors.As(err, &coded) || coded.Code != "SERVE_MANIFEST_CONFLICT" || coded.Context["project_id"] != "created" {
		t.Fatalf("expected recoverable created project, got %v", err)
	}
	manifest, current, err := workspace.ReadManifestSnapshot(root)
	if err != nil || manifest.Workspace.Name != "edited-during-creation" || manifest.Env != nil {
		t.Fatalf("binding overwrote external edits: %+v %v", manifest, err)
	}
	result, err := service.BindWorkspace(context.Background(), BindWorkspaceInput{Scope: scope, ProjectID: "created", Revision: current})
	if err != nil || result.ProjectID != "created" || creates != 1 {
		t.Fatalf("recovery created another project: %+v %v, creations=%d", result, err, creates)
	}
}

func TestSameNamedWorkspacesCreateDistinctProjectsAndRetryUsesBinding(t *testing.T) {
	var mu sync.Mutex
	names := map[string]string{}
	values := map[string]string{}
	writes := map[string]int{}
	failSave := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost && r.URL.Path == "/api/v2/workspace" {
			var body struct {
				Name string `json:"projectName"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if _, exists := names[body.Name]; exists {
				w.WriteHeader(409)
				fmt.Fprint(w, `{"message":"name already exists"}`)
				return
			}
			id := fmt.Sprintf("remote-%d", len(names)+1)
			names[body.Name] = id
			_ = json.NewEncoder(w).Encode(map[string]any{"project": map[string]string{"id": id, "name": body.Name}})
			return
		}
		if r.Method == http.MethodGet {
			id := r.URL.Query().Get("workspaceId")
			if value, ok := values[id]; ok {
				_ = json.NewEncoder(w).Encode(map[string]any{"secret": map[string]string{"secretKey": "TOKEN", "secretValue": value}})
				return
			}
			w.WriteHeader(404)
			fmt.Fprint(w, `{"message":"not found"}`)
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		id, _ := body["workspaceId"].(string)
		if id == "" {
			t.Errorf("missing project ID: %v", body)
			w.WriteHeader(400)
			return
		}
		if failSave {
			w.WriteHeader(403)
			fmt.Fprint(w, `{"message":"forbidden"}`)
			return
		}
		value, _ := body["secretValue"].(string)
		values[id] = value
		writes[id]++
		_ = json.NewEncoder(w).Encode(map[string]any{"secret": map[string]string{"secretKey": "TOKEN", "secretValue": value}})
	}))
	defer server.Close()
	mockSession(t, server.URL)
	service := newTestService(t)
	scopes := []execution.Scope{unboundScope(t), unboundScope(t)}
	ids := []string{}
	for index, scope := range scopes {
		binding, err := service.BindWorkspace(context.Background(), BindWorkspaceInput{Scope: scope, Create: true})
		if err != nil {
			t.Fatal(err)
		}
		plan, err := service.PlanSet(PlanSetInput{Scope: scope})
		if err != nil {
			t.Fatal(err)
		}
		_, err = service.Set(context.Background(), SetInput{Plan: plan, Key: "TOKEN", Value: fmt.Sprint(index)})
		if err != nil {
			t.Fatal(err)
		}
		if !binding.Created {
			t.Fatalf("binding=%+v", binding)
		}
		ids = append(ids, binding.ProjectID)
		if index == 1 && (binding.RequestedName != "demo" || !strings.HasPrefix(binding.ProjectName, "demo-")) {
			t.Fatalf("collision metadata=%+v", binding)
		}
		again, err := service.BindWorkspace(context.Background(), BindWorkspaceInput{Scope: scope, Create: true})
		if err != nil || again.Created || again.ProjectID != binding.ProjectID {
			t.Fatalf("repeat binding=%+v,%v", again, err)
		}
		if _, err := service.Set(context.Background(), SetInput{Plan: plan, Key: "TOKEN", Value: fmt.Sprint(index)}); err != nil {
			t.Fatal(err)
		}
	}
	if ids[0] == ids[1] || len(names) != 2 || writes[ids[0]] != 1 || writes[ids[1]] != 1 {
		t.Fatalf("project routing: ids=%v names=%v writes=%v", ids, names, writes)
	}
	// A copied manifest deliberately shares its saved ID. Overwriting requires
	// confirmation and names that actual target, even in another local folder.
	copyScope := unboundScope(t)
	original, err := os.ReadFile(workspace.ManifestPath(scopes[0].WorkingDirectory()))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(workspace.ManifestPath(copyScope.WorkingDirectory()), original, 0644); err != nil {
		t.Fatal(err)
	}
	copyPlan, err := service.PlanSet(PlanSetInput{Scope: copyScope})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Set(context.Background(), SetInput{Plan: copyPlan, Key: "TOKEN", Value: "changed"})
	var overwrite *output.Error
	if !errors.As(err, &overwrite) || overwrite.Code != "ENV_SET_OVERWRITE_REQUIRED" || overwrite.Context["project_id"] != ids[0] || !strings.Contains(err.Error(), ids[0]) {
		t.Fatalf("copied binding error=%v", err)
	}
	if len(names) != 2 || writes[ids[0]] != 1 {
		t.Fatal("copy unexpectedly created or changed a project")
	}
	// A failed variable write must preserve a successfully created binding.
	mu.Lock()
	failSave = true
	mu.Unlock()
	scope := unboundScope(t)
	bound, err := service.BindWorkspace(context.Background(), BindWorkspaceInput{Scope: scope, Create: true})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := service.PlanSet(PlanSetInput{Scope: scope})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Set(context.Background(), SetInput{Plan: plan, Key: "TOKEN", Value: "third"})
	var coded *output.Error
	if !errors.As(err, &coded) {
		t.Fatalf("variable write failure=%v", err)
	}
	manifest, readErr := workspace.ReadManifest(scope.WorkingDirectory())
	if readErr != nil || manifest.Env.ProjectID != bound.ProjectID {
		t.Fatalf("failed variable write changed binding: %+v %v", manifest, readErr)
	}
	mu.Lock()
	failSave = false
	mu.Unlock()
	retried, err := service.Set(context.Background(), SetInput{Plan: plan, Key: "TOKEN", Value: "third"})
	if err != nil || len(names) != 3 {
		t.Fatalf("retry=%+v,%v; projects=%v", retried, err, names)
	}
}

func TestBindingInitializationIsSerializedAndMalformedConfigIsRejected(t *testing.T) {
	mockSession(t, "http://127.0.0.1:1")
	service := newTestService(t)
	scope := unboundScope(t)
	calls := 0
	service.initInfisical = func(_ context.Context, root string, _ infisical.InitInput) (*infisical.InitResult, error) {
		calls++
		manifest, err := workspace.ReadManifest(root)
		if err != nil {
			return nil, err
		}
		manifest.Env = &workspace.EnvironmentConfig{ProjectID: "remote", Environments: []string{"dev", "staging", "prod"}}
		if err := workspace.WriteManifest(root, manifest); err != nil {
			return nil, err
		}
		return &infisical.InitResult{ProjectID: "remote", ProjectName: "demo", Created: true}, nil
	}
	var wg sync.WaitGroup
	for range 3 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := service.BindWorkspace(context.Background(), BindWorkspaceInput{Scope: scope, Create: true}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if calls != 1 {
		t.Fatalf("initialized %d times", calls)
	}
	if err := os.WriteFile(workspace.ManifestPath(scope.WorkingDirectory()), []byte("broken"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := service.BindWorkspace(context.Background(), BindWorkspaceInput{Scope: scope, Create: true}); err == nil {
		t.Fatal("malformed config accepted")
	}
	if calls != 1 {
		t.Fatal("malformed config triggered creation")
	}
}
