package serve

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	workspacecore "github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

func TestCreateProjectInSelectedWorkspace(t *testing.T) {
	registry := newRegistryService(t)
	launchRoot := seedRegistryWorkspace(t, "launch", "Launch", "")
	selectedRoot := seedRegistryWorkspace(t, "selected", "Selected", "")
	selected := observeRegistryWorkspace(t, registry, selectedRoot)
	handler := newRegistryMux(t, launchRoot, registry)
	before := snapshotRepositoryTree(t, launchRoot)
	path := "/api/workspaces/" + selected.EntryID + "/projects"
	response := registryRequest(t, handler, http.MethodPost, path, strings.NewReader(`{"name":"web","templateId":"react-spa"}`))
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	var created createProjectResponse
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Name != "web" || created.RelativeDir != "apps/web" || created.TemplateID != "react-spa" {
		t.Fatalf("created = %#v", created)
	}
	if _, err := os.Stat(filepath.Join(selectedRoot, "apps/web/package.json")); err != nil {
		t.Fatal(err)
	}
	manifest, err := workspacecore.ReadManifest(selectedRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Projects) != 1 || manifest.Projects[0].Name != "web" {
		t.Fatalf("manifest = %#v", manifest)
	}
	assertRepositoryUnchanged(t, launchRoot, before)
	before = snapshotRepositoryTree(t, selectedRoot)
	for _, templateID := range []string{"react-spa", "go-api"} {
		response = registryRequest(t, handler, http.MethodPost, path, strings.NewReader(`{"name":"web","templateId":"`+templateID+`"}`))
		if response.Code != http.StatusConflict {
			t.Fatalf("duplicate status = %d: %s", response.Code, response.Body.String())
		}
		assertRepositoryUnchanged(t, selectedRoot, before)
	}
}

func TestCreateProjectRejectsInvalidRequestsWithoutWrites(t *testing.T) {
	registry := newRegistryService(t)
	root := seedRegistryWorkspace(t, "selected", "Selected", "")
	selected := observeRegistryWorkspace(t, registry, root)
	handler := newRegistryMux(t, root, registry)
	before := snapshotRepositoryTree(t, root)
	for _, test := range []struct {
		body   string
		status int
	}{
		{`{"name":"../escape","templateId":"react-spa"}`, 400},
		{`{"name":"web/escape","templateId":"react-spa"}`, 400},
		{`{"name":"","templateId":"react-spa"}`, 400},
		{`{"name":"web","templateId":""}`, 400},
		{`{"name":"web","templateId":"unknown"}`, 404},
		{`{"name":"web","templateId":"react-spa","root":"/tmp"}`, 400},
		{`{"name":"web","templateId":"react-spa"} {}`, 400},
	} {
		response := registryRequest(t, handler, http.MethodPost, "/api/workspaces/"+selected.EntryID+"/projects", strings.NewReader(test.body))
		if response.Code != test.status {
			t.Errorf("%s: status = %d: %s", test.body, response.Code, response.Body.String())
		}
		assertRepositoryUnchanged(t, root, before)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/workspaces/"+selected.EntryID+"/projects", strings.NewReader(`{"name":"web","templateId":"react-spa"}`))
	request.Host = registryTestHost
	request.Header.Set("Origin", "http://external.example")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("origin status = %d", response.Code)
	}
	assertRepositoryUnchanged(t, root, before)
}

func TestCreateProjectRejectsUnknownAndConflictingWorkspace(t *testing.T) {
	registry := newRegistryService(t)
	root := seedRegistryWorkspace(t, "copied-id", "A", "")
	other := seedRegistryWorkspace(t, "copied-id", "B", "")
	selected := observeRegistryWorkspace(t, registry, root)
	observeRegistryWorkspace(t, registry, other)
	handler := newRegistryMux(t, root, registry)
	before := snapshotRepositoryTree(t, root)
	for entry, status := range map[string]int{selected.EntryID: 409, "unknown": 404} {
		response := registryRequest(t, handler, http.MethodPost, "/api/workspaces/"+entry+"/projects", strings.NewReader(`{"name":"web","templateId":"react-spa"}`))
		if response.Code != status {
			t.Fatalf("status = %d: %s", response.Code, response.Body.String())
		}
	}
	assertRepositoryUnchanged(t, root, before)
}

func TestCreateProjectConcurrentDuplicate(t *testing.T) {
	registry := newRegistryService(t)
	root := seedRegistryWorkspace(t, "selected", "Selected", "")
	selected := observeRegistryWorkspace(t, registry, root)
	handler := newRegistryMux(t, root, registry)
	statuses := make(chan int, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			response := registryRequest(t, handler, http.MethodPost, "/api/workspaces/"+selected.EntryID+"/projects", strings.NewReader(`{"name":"web","templateId":"react-spa"}`))
			statuses <- response.Code
		}()
	}
	wg.Wait()
	first, second := <-statuses, <-statuses
	if !((first == 201 && second == 409) || (first == 409 && second == 201)) {
		t.Fatalf("statuses = %d, %d", first, second)
	}
	manifest, err := workspacecore.ReadManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Projects) != 1 {
		t.Fatalf("projects = %#v", manifest.Projects)
	}
}

func TestProjectTemplatesLocalized(t *testing.T) {
	original := i18n.Active()
	t.Cleanup(func() { _ = i18n.Init(original) })
	for _, locale := range []string{"zh-CN", "en-US"} {
		_ = i18n.Init(locale)
		handler := newRegistryMux(t, "", nil)
		response := registryRequest(t, handler, http.MethodGet, "/api/project-templates", nil)
		if response.Code != http.StatusOK {
			t.Fatal(response.Body.String())
		}
		var result struct {
			Templates []projectTemplate `json:"templates"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if len(result.Templates) == 0 {
			t.Fatal("no templates")
		}
		for _, entry := range result.Templates {
			if entry.Directory == "" || entry.Name != i18n.T("template."+entry.ID+".name") {
				t.Fatalf("template = %#v", entry)
			}
		}
	}
}
