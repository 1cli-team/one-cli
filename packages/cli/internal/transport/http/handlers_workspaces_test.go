package serve

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	registrylocal "github.com/torchstellar-team/one-cli/packages/cli/internal/adapters/workspaceregistry/local"
	workspaceapp "github.com/torchstellar-team/one-cli/packages/cli/internal/application/workspace"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/profile"
	workspacecore "github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
)

const registryTestHost = "one.test"

func seedRegistryWorkspace(t *testing.T, id, name, projectName string) string {
	t.Helper()
	root := t.TempDir()
	manifest := &workspacecore.Manifest{
		Version:   workspacecore.ManifestVersion,
		Workspace: &workspacecore.ManifestWorkspace{ID: id, Name: name},
	}
	if projectName != "" {
		manifest.Projects = []workspacecore.ManifestProject{{
			Name: projectName, RelativeDir: "apps/" + projectName,
			TemplateID: "react-spa", Toolchain: "node",
		}}
	}
	if err := workspacecore.WriteManifest(root, manifest); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}
	return root
}

func newRegistryService(t *testing.T) *workspaceapp.RegistryService {
	t.Helper()
	store := registrylocal.NewAt(filepath.Join(t.TempDir(), "workspaces.json"))
	service, err := workspaceapp.NewRegistryService(store)
	if err != nil {
		t.Fatalf("NewRegistryService: %v", err)
	}
	return service
}

func observeRegistryWorkspace(
	t *testing.T,
	service *workspaceapp.RegistryService,
	root string,
) workspaceapp.RegisteredWorkspace {
	t.Helper()
	registered, err := service.Observe(context.Background(), root, "serve")
	if err != nil {
		t.Fatalf("Observe(%q): %v", root, err)
	}
	return registered
}

func newRegistryMux(
	t *testing.T,
	launchRoot string,
	registry *workspaceapp.RegistryService,
) http.Handler {
	t.Helper()
	withIsolatedConfig(t)
	return BuildMux(MuxOpts{
		UIDisabled:      true,
		ExpectedHosts:   map[string]struct{}{registryTestHost: {}},
		SelfOrigin:      "http://" + registryTestHost,
		WorkspaceRoot:   launchRoot,
		RegistryService: registry,
	})
}

func registryRequest(
	t *testing.T,
	handler http.Handler,
	method, path string,
	body io.Reader,
) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, body)
	req.Host = registryTestHost
	if method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions {
		req.Header.Set("Origin", "http://"+registryTestHost)
		req.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder
}

func TestWorkspacesListMarksLaunchWorkspaceCurrent(t *testing.T) {
	registry := newRegistryService(t)
	launchRoot := seedRegistryWorkspace(t, "launch-id", "Launch", "launch-web")
	otherRoot := seedRegistryWorkspace(t, "other-id", "Other", "other-web")
	launch := observeRegistryWorkspace(t, registry, launchRoot)
	observeRegistryWorkspace(t, registry, otherRoot)

	recorder := registryRequest(t, newRegistryMux(t, launchRoot, registry),
		http.MethodGet, "/api/workspaces", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", recorder.Code, recorder.Body.String())
	}
	var response workspaceapp.WorkspaceRegistryResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode: %v; body = %s", err, recorder.Body.String())
	}
	if response.Schema != workspaceapp.WorkspaceRegistrySchema {
		t.Fatalf("schema = %q", response.Schema)
	}
	if response.CurrentEntryID != launch.EntryID {
		t.Fatalf("currentEntryId = %q; want %q", response.CurrentEntryID, launch.EntryID)
	}
	if len(response.Workspaces) != 2 {
		t.Fatalf("workspaces = %#v", response.Workspaces)
	}
}

func TestWorkspacesSelectedOverviewAndProject(t *testing.T) {
	registry := newRegistryService(t)
	launchRoot := seedRegistryWorkspace(t, "launch-id", "Launch", "launch-web")
	selectedRoot := seedRegistryWorkspace(t, "selected-id", "Selected", "selected-web")
	observeRegistryWorkspace(t, registry, launchRoot)
	selected := observeRegistryWorkspace(t, registry, selectedRoot)
	handler := newRegistryMux(t, launchRoot, registry)

	recorder := registryRequest(t, handler, http.MethodGet,
		"/api/workspaces/"+selected.EntryID+"/overview", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("overview status = %d; body = %s", recorder.Code, recorder.Body.String())
	}
	var overview workspacecore.Overview
	if err := json.Unmarshal(recorder.Body.Bytes(), &overview); err != nil {
		t.Fatalf("decode overview: %v", err)
	}
	if overview.Workspace == nil || overview.Workspace.Name != "Selected" ||
		len(overview.Projects) != 1 || overview.Projects[0].Name != "selected-web" {
		t.Fatalf("overview came from wrong root: %#v", overview)
	}

	recorder = registryRequest(t, handler, http.MethodGet,
		"/api/workspaces/"+selected.EntryID+"/projects/selected-web", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("project status = %d; body = %s", recorder.Code, recorder.Body.String())
	}
	var project workspaceSettingsWire
	if err := json.Unmarshal(recorder.Body.Bytes(), &project); err != nil {
		t.Fatalf("decode project: %v", err)
	}
	if project.Project.Name != "selected-web" {
		t.Fatalf("project came from wrong root: %#v", project.Project)
	}
}

func TestWorkspacesProfileBindingUsesResolvedRootAndLeavesManifestsUnchanged(t *testing.T) {
	registry := newRegistryService(t)
	launchRoot := seedRegistryWorkspace(t, "launch-id", "Launch", "launch-web")
	selectedRoot := seedRegistryWorkspace(t, "selected-id", "Selected", "selected-web")
	for _, root := range []string{launchRoot, selectedRoot} {
		manifest, err := workspacecore.ReadManifest(root)
		if err != nil {
			t.Fatal(err)
		}
		manifest.Domains = &workspacecore.WorkspaceDomains{
			Env: &workspacecore.BackendRef{Kind: workspacecore.EnvBackendInfisical},
		}
		if err := workspacecore.WriteManifest(root, manifest); err != nil {
			t.Fatal(err)
		}
	}
	observeRegistryWorkspace(t, registry, launchRoot)
	selected := observeRegistryWorkspace(t, registry, selectedRoot)
	handler := newRegistryMux(t, launchRoot, registry)
	if _, err := profile.Upsert(profile.DomainEnv, workspacecore.EnvBackendInfisical, "work", profile.Profile{
		Backend: workspacecore.EnvBackendInfisical,
		Infisical: &profile.InfisicalProfile{
			Credentials: &profile.InfisicalCredentials{
				ClientID: "client", ClientSecret: "scoped-secret-must-not-leak",
			},
		},
	}, true); err != nil {
		t.Fatal(err)
	}
	selectedTreeBefore := snapshotRepositoryTree(t, selectedRoot)
	launchTreeBefore := snapshotRepositoryTree(t, launchRoot)
	selectedBefore, err := os.ReadFile(filepath.Join(selectedRoot, workspacecore.ManifestFilename))
	if err != nil {
		t.Fatal(err)
	}
	launchBefore, err := os.ReadFile(filepath.Join(launchRoot, workspacecore.ManifestFilename))
	if err != nil {
		t.Fatal(err)
	}

	path := "/api/workspaces/" + selected.EntryID + "/profile-bindings/env?root=" + launchRoot
	recorder := registryRequest(t, handler, http.MethodPut, path,
		strings.NewReader(`{"profile":"work"}`))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", recorder.Code, recorder.Body.String())
	}
	var settings workspaceProfileSettingsWire
	if err := json.Unmarshal(recorder.Body.Bytes(), &settings); err != nil {
		t.Fatal(err)
	}
	if settings.Root != selected.Root || settings.Profile == nil ||
		settings.Profile.Name != "work" || settings.Profile.Source != "workspace" {
		t.Fatalf("scoped settings = %#v", settings)
	}
	if strings.Contains(recorder.Body.String(), "scoped-secret-must-not-leak") ||
		strings.Contains(recorder.Body.String(), "client") {
		t.Fatalf("scoped response leaked credentials: %s", recorder.Body.String())
	}
	config, _, err := profile.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := config.Workspaces["selected-id"].Profiles["env/infisical"]; got != "work" {
		t.Fatalf("selected workspace binding = %q", got)
	}
	if _, exists := config.Workspaces["launch-id"]; exists {
		t.Fatal("query root injection bound the launch workspace")
	}
	selectedAfter, err := os.ReadFile(filepath.Join(selectedRoot, workspacecore.ManifestFilename))
	if err != nil {
		t.Fatal(err)
	}
	launchAfter, err := os.ReadFile(filepath.Join(launchRoot, workspacecore.ManifestFilename))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(selectedAfter, selectedBefore) || !bytes.Equal(launchAfter, launchBefore) {
		t.Fatal("scoped workspace profile binding changed a manifest")
	}
	assertRepositoryUnchanged(t, selectedRoot, selectedTreeBefore)
	assertRepositoryUnchanged(t, launchRoot, launchTreeBefore)

	readRecorder := registryRequest(t, handler, http.MethodGet,
		"/api/workspaces/"+selected.EntryID+"/profile-bindings/env", nil)
	if readRecorder.Code != http.StatusOK {
		t.Fatalf("GET status = %d; body = %s", readRecorder.Code, readRecorder.Body.String())
	}
	unboundRecorder := registryRequest(t, handler, http.MethodPut,
		"/api/workspaces/"+selected.EntryID+"/profile-bindings/env",
		strings.NewReader(`{"profile":""}`))
	if unboundRecorder.Code != http.StatusOK {
		t.Fatalf("unbind status = %d; body = %s", unboundRecorder.Code, unboundRecorder.Body.String())
	}
	selectedAfterUnbind, err := os.ReadFile(filepath.Join(selectedRoot, workspacecore.ManifestFilename))
	if err != nil {
		t.Fatal(err)
	}
	launchAfterUnbind, err := os.ReadFile(filepath.Join(launchRoot, workspacecore.ManifestFilename))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(selectedAfterUnbind, selectedBefore) || !bytes.Equal(launchAfterUnbind, launchBefore) {
		t.Fatal("scoped workspace profile unbind changed a manifest")
	}
	assertRepositoryUnchanged(t, selectedRoot, selectedTreeBefore)
	assertRepositoryUnchanged(t, launchRoot, launchTreeBefore)
}

func TestWorkspacesLegacyProjectMutationIsReadOnlyWithoutResolvingBodyRoot(t *testing.T) {
	registry := newRegistryService(t)
	launchRoot := seedRegistryWorkspace(t, "launch-id", "Launch", "launch-web")
	selectedRoot := seedRegistryWorkspace(t, "selected-id", "Selected", "selected-web")
	observeRegistryWorkspace(t, registry, launchRoot)
	selected := observeRegistryWorkspace(t, registry, selectedRoot)
	handler := newRegistryMux(t, launchRoot, registry)
	before, err := workspacecore.ReadManifest(selectedRoot)
	if err != nil {
		t.Fatal(err)
	}
	beforeBuildVersion := before.Projects[0].BuildVersion

	recorder := registryRequest(t, handler, http.MethodPut,
		"/api/workspaces/"+selected.EntryID+"/projects/selected-web",
		strings.NewReader(`{"buildVersion":"9.9.9","root":"`+launchRoot+`"}`))
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d; want 409; body = %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"code":"SERVE_REPOSITORY_READ_ONLY"`) {
		t.Fatalf("unexpected error contract: %s", recorder.Body.String())
	}
	manifest, err := workspacecore.ReadManifest(selectedRoot)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Projects[0].BuildVersion != beforeBuildVersion {
		t.Fatal("rejected root injection changed the selected workspace")
	}
}

func TestWorkspacesProjectProfileBindingUsesResolvedRootAndEnvironment(t *testing.T) {
	registry := newRegistryService(t)
	launchRoot := seedRegistryWorkspace(t, "launch-id", "Launch", "launch-web")
	selectedRoot := seedRegistryWorkspace(t, "selected-id", "Selected", "selected-web")
	manifest, err := workspacecore.ReadManifest(selectedRoot)
	if err != nil {
		t.Fatal(err)
	}
	manifest.Domains = &workspacecore.WorkspaceDomains{
		Env: &workspacecore.BackendRef{Kind: workspacecore.EnvBackendInfisical},
	}
	if err := workspacecore.WriteManifest(selectedRoot, manifest); err != nil {
		t.Fatal(err)
	}
	observeRegistryWorkspace(t, registry, launchRoot)
	selected := observeRegistryWorkspace(t, registry, selectedRoot)
	handler := newRegistryMux(t, launchRoot, registry)
	if _, err := profile.Upsert(profile.DomainEnv, workspacecore.EnvBackendInfisical, "work", profile.Profile{
		Backend: workspacecore.EnvBackendInfisical,
		Infisical: &profile.InfisicalProfile{
			Credentials: &profile.InfisicalCredentials{
				ClientID: "client", ClientSecret: "plural-secret-must-not-leak",
			},
		},
	}, true); err != nil {
		t.Fatal(err)
	}
	beforeSelected := snapshotRepositoryTree(t, selectedRoot)
	beforeLaunch := snapshotRepositoryTree(t, launchRoot)
	path := "/api/workspaces/" + selected.EntryID +
		"/projects/selected-web/profile-bindings/env?env=preview"

	recorder := registryRequest(t, handler, http.MethodPut, path,
		strings.NewReader(`{"profile":"work"}`))
	if recorder.Code != http.StatusOK {
		t.Fatalf("bind status = %d; body = %s", recorder.Code, recorder.Body.String())
	}
	var settings workspaceSettingsWire
	if err := json.Unmarshal(recorder.Body.Bytes(), &settings); err != nil {
		t.Fatal(err)
	}
	if settings.Root != selected.Root || settings.Environment != "preview" ||
		settings.Project.Environment.SelectedProfile != "work" ||
		settings.Project.Environment.Profile == nil ||
		settings.Project.Environment.Profile.Source != "workspace-project-environment" {
		t.Fatalf("settings = %#v", settings)
	}
	if strings.Contains(recorder.Body.String(), "plural-secret-must-not-leak") {
		t.Fatalf("response leaked credentials: %s", recorder.Body.String())
	}
	assertRepositoryUnchanged(t, selectedRoot, beforeSelected)
	assertRepositoryUnchanged(t, launchRoot, beforeLaunch)

	recorder = registryRequest(t, handler, http.MethodPut, path,
		strings.NewReader(`{"profile":""}`))
	if recorder.Code != http.StatusOK {
		t.Fatalf("unbind status = %d; body = %s", recorder.Code, recorder.Body.String())
	}
	assertRepositoryUnchanged(t, selectedRoot, beforeSelected)
	assertRepositoryUnchanged(t, launchRoot, beforeLaunch)
}

func TestWorkspacesLegacyMutationPathsAreReadOnlyEvenForIdentityConflict(t *testing.T) {
	registry := newRegistryService(t)
	rootA := seedRegistryWorkspace(t, "copied-id", "Copy A", "web")
	rootB := seedRegistryWorkspace(t, "copied-id", "Copy B", "web")
	conflict := observeRegistryWorkspace(t, registry, rootA)
	observeRegistryWorkspace(t, registry, rootB)
	handler := newRegistryMux(t, "", registry)
	beforeA := snapshotRepositoryTree(t, rootA)
	beforeB := snapshotRepositoryTree(t, rootB)

	for _, suffix := range []string{"", "/environment", "/deploy", "/container", "/settings/deploy", "/settings/container"} {
		path := "/api/workspaces/" + conflict.EntryID + "/projects/web" + suffix
		recorder := registryRequest(t, handler, http.MethodPut, path, strings.NewReader(`{}`))
		if recorder.Code != http.StatusConflict ||
			!strings.Contains(recorder.Body.String(), `"code":"SERVE_REPOSITORY_READ_ONLY"`) {
			t.Fatalf("%s status = %d; body = %s", path, recorder.Code, recorder.Body.String())
		}
	}
	assertRepositoryUnchanged(t, rootA, beforeA)
	assertRepositoryUnchanged(t, rootB, beforeB)
}

func TestWorkspacesResolveErrorsHaveStableStatuses(t *testing.T) {
	registry := newRegistryService(t)
	missingRoot := seedRegistryWorkspace(t, "missing-id", "Missing", "web")
	missing := observeRegistryWorkspace(t, registry, missingRoot)
	if err := os.Remove(filepath.Join(missingRoot, workspacecore.ManifestFilename)); err != nil {
		t.Fatal(err)
	}

	conflictRootA := seedRegistryWorkspace(t, "copied-id", "Copy A", "web-a")
	conflictRootB := seedRegistryWorkspace(t, "copied-id", "Copy B", "web-b")
	conflictBeforeA, err := os.ReadFile(filepath.Join(conflictRootA, workspacecore.ManifestFilename))
	if err != nil {
		t.Fatal(err)
	}
	conflictBeforeB, err := os.ReadFile(filepath.Join(conflictRootB, workspacecore.ManifestFilename))
	if err != nil {
		t.Fatal(err)
	}
	conflict := observeRegistryWorkspace(t, registry, conflictRootA)
	observeRegistryWorkspace(t, registry, conflictRootB)
	handler := newRegistryMux(t, "", registry)

	for _, test := range []struct {
		name       string
		entryID    string
		wantStatus int
	}{
		{name: "unknown", entryID: "wsr_unknown", wantStatus: http.StatusNotFound},
		{name: "missing", entryID: missing.EntryID, wantStatus: http.StatusConflict},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := registryRequest(t, handler, http.MethodGet,
				"/api/workspaces/"+test.entryID+"/overview", nil)
			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d; want %d; body = %s",
					recorder.Code, test.wantStatus, recorder.Body.String())
			}
		})
	}

	readRecorder := registryRequest(t, handler, http.MethodGet,
		"/api/workspaces/"+conflict.EntryID+"/overview", nil)
	if readRecorder.Code != http.StatusOK {
		t.Fatalf("identity-conflict read status = %d; body = %s",
			readRecorder.Code, readRecorder.Body.String())
	}
	profileReadRecorder := registryRequest(t, handler, http.MethodGet,
		"/api/workspaces/"+conflict.EntryID+"/profile-bindings/env", nil)
	if profileReadRecorder.Code != http.StatusOK {
		t.Fatalf("identity-conflict profile read status = %d; body = %s",
			profileReadRecorder.Code, profileReadRecorder.Body.String())
	}
	writeRecorder := registryRequest(t, handler, http.MethodPut,
		"/api/workspaces/"+conflict.EntryID+"/profile-bindings/env",
		strings.NewReader(`{"profile":"work"}`))
	if writeRecorder.Code != http.StatusConflict {
		t.Fatalf("identity-conflict mutation status = %d; want 409; body = %s",
			writeRecorder.Code, writeRecorder.Body.String())
	}
	conflictAfterA, err := os.ReadFile(filepath.Join(conflictRootA, workspacecore.ManifestFilename))
	if err != nil {
		t.Fatal(err)
	}
	conflictAfterB, err := os.ReadFile(filepath.Join(conflictRootB, workspacecore.ManifestFilename))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(conflictAfterA, conflictBeforeA) || !bytes.Equal(conflictAfterB, conflictBeforeB) {
		t.Fatal("identity-conflict profile request changed a manifest")
	}
}

func TestWorkspacesForgetOnlyRemovesRegistryEntry(t *testing.T) {
	registry := newRegistryService(t)
	root := seedRegistryWorkspace(t, "forget-id", "Forget", "web")
	registered := observeRegistryWorkspace(t, registry, root)
	handler := newRegistryMux(t, root, registry)

	recorder := registryRequest(t, handler, http.MethodDelete,
		"/api/workspaces/"+registered.EntryID, nil)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d; body = %s", recorder.Code, recorder.Body.String())
	}
	if _, err := os.Stat(filepath.Join(root, workspacecore.ManifestFilename)); err != nil {
		t.Fatalf("Forget changed the workspace on disk: %v", err)
	}
	response, err := registry.List(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Workspaces) != 0 {
		t.Fatalf("registry still contains forgotten entry: %#v", response.Workspaces)
	}
}

func TestWorkspacesNilRegistryKeepsLegacyRoutesAndReturnsEmptyList(t *testing.T) {
	root := seedRegistryWorkspace(t, "legacy-id", "Legacy", "web")
	handler := newRegistryMux(t, root, nil)

	recorder := registryRequest(t, handler, http.MethodGet, "/api/workspaces", nil)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"workspaces": []`) {
		t.Fatalf("list status = %d; body = %s", recorder.Code, recorder.Body.String())
	}

	recorder = registryRequest(t, handler, http.MethodGet, "/api/workspace/overview", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("legacy overview status = %d; body = %s", recorder.Code, recorder.Body.String())
	}
	var overview workspacecore.Overview
	if err := json.Unmarshal(recorder.Body.Bytes(), &overview); err != nil {
		t.Fatal(err)
	}
	if overview.Workspace == nil || overview.Workspace.Name != "Legacy" {
		t.Fatalf("legacy overview changed: %#v", overview)
	}
}
