package serve

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	workspacecore "github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
)

func TestWorkspaceBindingUsesSelectedWorkspaceAndRejectsStaleRequests(t *testing.T) {
	registry := newRegistryService(t)
	launch := seedRegistryWorkspace(t, "launch", "Launch", "")
	root := seedRegistryWorkspace(t, "selected", "Selected", "")
	manifest, err := workspacecore.ReadManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	manifest.Env = &workspacecore.EnvironmentConfig{ProjectID: "selected-project", Environments: []string{"dev"}}
	if err := workspacecore.WriteManifest(root, manifest); err != nil {
		t.Fatal(err)
	}
	selected := observeRegistryWorkspace(t, registry, root)
	handler := newRegistryMux(t, launch, registry)
	before := snapshotRepositoryTree(t, launch)
	selectedBefore := snapshotRepositoryTree(t, root)
	_, revision, err := workspacecore.ReadManifestSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	path := "/api/workspaces/" + selected.EntryID + "/environment/bind"
	for _, test := range []struct {
		body   string
		status int
	}{
		{`{"create":true}`, http.StatusBadRequest},
		{`{"revision":"stale","create":true}`, http.StatusConflict},
		{fmt.Sprintf(`{"revision":%q,"create":true,"projectId":"other"}`, revision), http.StatusBadRequest},
		{fmt.Sprintf(`{"revision":%q,"create":false}`, revision), http.StatusBadRequest},
		{fmt.Sprintf(`{"revision":%q,"create":true}`, revision), http.StatusOK},
	} {
		response := registryRequest(t, handler, http.MethodPost, path, strings.NewReader(test.body))
		if response.Code != test.status {
			t.Fatalf("%s: status=%d body=%s", test.body, response.Code, response.Body.String())
		}
		if test.status == http.StatusOK && !strings.Contains(response.Body.String(), `"project_id": "selected-project"`) {
			t.Fatalf("bound wrong workspace: %s", response.Body.String())
		}
		assertRepositoryUnchanged(t, launch, before)
		assertRepositoryUnchanged(t, root, selectedBefore)
	}
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"create":true}`))
	request.Host = registryTestHost
	request.Header.Set("Origin", "http://external.example")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("cross-origin binding status=%d", response.Code)
	}
}
