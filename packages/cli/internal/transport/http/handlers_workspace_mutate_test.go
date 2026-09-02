package serve

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/profile"
	workspacecore "github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
)

const workspaceTestHost = "dashboard.test"

func seedWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	deployConfig, err := json.Marshal(map[string]string{"projectId": "prj_web", "env": "prod"})
	if err != nil {
		t.Fatal(err)
	}
	manifest := &workspacecore.Manifest{
		Version:      workspacecore.ManifestVersion,
		Workspace:    &workspacecore.ManifestWorkspace{ID: "demo", Name: "demo"},
		Environments: &workspacecore.Environments{Names: []string{"dev", "staging", "prod"}, Default: "dev"},
		Domains: &workspacecore.WorkspaceDomains{
			Env: &workspacecore.BackendRef{Kind: workspacecore.EnvBackendInfisical},
		},
		Projects: []workspacecore.ManifestProject{{
			Name: "web", RelativeDir: "apps/web", TemplateID: "react-spa", Toolchain: "node",
			Domains: &workspacecore.ProjectDomains{
				Container: &workspacecore.ProjectContainerOverride{Kind: "ghcr", Image: "ghcr.io/acme/web:latest"},
				Deploy:    &workspacecore.ProjectDeployBackend{Kind: "vercel", Config: deployConfig},
			},
		}},
	}
	if err := workspacecore.WriteManifest(root, manifest); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("repository content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func newWorkspaceMux(t *testing.T, root string) http.Handler {
	t.Helper()
	withIsolatedConfig(t)
	return BuildMux(MuxOpts{
		UIDisabled:    true,
		ExpectedHosts: map[string]struct{}{workspaceTestHost: {}},
		SelfOrigin:    "http://" + workspaceTestHost,
		WorkspaceRoot: root,
	})
}

func workspaceRequest(
	t *testing.T,
	handler http.Handler,
	method, path string,
	body io.Reader,
) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, body)
	request.Host = workspaceTestHost
	if method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions {
		request.Header.Set("Origin", "http://"+workspaceTestHost)
		request.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func seedDashboardProfiles(t *testing.T) {
	t.Helper()
	if _, err := profile.Upsert(profile.DomainEnv, "infisical", "work", profile.Profile{
		Backend: "infisical",
		Infisical: &profile.InfisicalProfile{
			SiteURL: "https://app.infisical.com",
			Credentials: &profile.InfisicalCredentials{
				ClientID: "client", ClientSecret: "secret-must-not-leak",
			},
		},
	}, true); err != nil {
		t.Fatal(err)
	}
	if _, err := profile.Upsert(profile.DomainContainer, "ghcr", "work", profile.Profile{
		Backend: "ghcr",
		Container: &profile.ContainerProfile{Credentials: &profile.ContainerCredentials{
			Username: "octo", Password: "container-secret-must-not-leak",
		}},
	}, true); err != nil {
		t.Fatal(err)
	}
	if _, err := profile.Upsert(profile.DomainDeploy, "vercel", "work", profile.Profile{
		Backend: "vercel",
		Vercel: &profile.VercelProfile{Credentials: &profile.VercelCredentials{
			APIToken: "deploy-secret-must-not-leak",
		}},
	}, true); err != nil {
		t.Fatal(err)
	}
}

func snapshotRepositoryTree(t *testing.T, root string) map[string][]byte {
	t.Helper()
	result := map[string][]byte{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		value, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		result[filepath.ToSlash(relative)] = value
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func assertRepositoryUnchanged(t *testing.T, root string, before map[string][]byte) {
	t.Helper()
	after := snapshotRepositoryTree(t, root)
	if len(after) != len(before) {
		t.Fatalf("repository file count changed: got %d want %d", len(after), len(before))
	}
	for path, expected := range before {
		if !bytes.Equal(after[path], expected) {
			t.Fatalf("repository file %q changed", path)
		}
	}
}

type workspaceProfileSettingsWire struct {
	Schema          string `json:"schema"`
	Root            string `json:"root"`
	Environment     string `json:"environment"`
	Revision        string `json:"revision"`
	Domain          string `json:"domain"`
	Backend         string `json:"backend"`
	Configurable    bool   `json:"configurable"`
	SelectedProfile string `json:"selectedProfile"`
	Profile         *struct {
		Name   string `json:"name"`
		Source string `json:"source"`
	} `json:"profile"`
}

type workspaceSettingsWire struct {
	Schema      string `json:"schema"`
	Root        string `json:"root"`
	Environment string `json:"environment"`
	Project     struct {
		Name        string `json:"name"`
		Environment struct {
			Backend         string `json:"backend"`
			SelectedProfile string `json:"selectedProfile"`
			Profile         *struct {
				Name   string `json:"name"`
				Source string `json:"source"`
			} `json:"profile"`
		} `json:"environment"`
		Container struct {
			Enabled         bool   `json:"enabled"`
			Backend         string `json:"backend"`
			SelectedProfile string `json:"selectedProfile"`
			Profile         *struct {
				Name   string `json:"name"`
				Source string `json:"source"`
			} `json:"profile"`
		} `json:"container"`
		Deploy struct {
			Backend         string `json:"backend"`
			SelectedProfile string `json:"selectedProfile"`
			Profile         *struct {
				Name   string `json:"name"`
				Source string `json:"source"`
			} `json:"profile"`
		} `json:"deploy"`
	} `json:"project"`
}

func TestWorkspaceEnvironmentProfileBindingIsEnvironmentAwareAndRepositoryReadOnly(t *testing.T) {
	root := seedWorkspace(t)
	handler := newWorkspaceMux(t, root)
	seedDashboardProfiles(t)
	before := snapshotRepositoryTree(t, root)

	recorder := workspaceRequest(t, handler, http.MethodPut,
		"/api/workspace/profile-bindings/env?env=preview", strings.NewReader(`{"profile":"work"}`))
	if recorder.Code != http.StatusOK {
		t.Fatalf("PUT status = %d; body = %s", recorder.Code, recorder.Body.String())
	}
	var settings workspaceProfileSettingsWire
	if err := json.Unmarshal(recorder.Body.Bytes(), &settings); err != nil {
		t.Fatal(err)
	}
	if settings.Environment != "preview" || settings.SelectedProfile != "work" ||
		settings.Revision == "" || settings.Profile == nil ||
		settings.Profile.Source != "workspace-environment" {
		t.Fatalf("settings = %#v", settings)
	}
	if strings.Contains(recorder.Body.String(), "must-not-leak") ||
		strings.Contains(recorder.Body.String(), "client") {
		t.Fatalf("response leaked credentials: %s", recorder.Body.String())
	}
	assertRepositoryUnchanged(t, root, before)
	overviewRecorder := workspaceRequest(t, handler, http.MethodGet,
		"/api/workspace/overview?env=preview", nil)
	if overviewRecorder.Code != http.StatusOK {
		t.Fatalf("overview status = %d; body = %s", overviewRecorder.Code, overviewRecorder.Body.String())
	}
	var overview workspacecore.Overview
	if err := json.Unmarshal(overviewRecorder.Body.Bytes(), &overview); err != nil {
		t.Fatal(err)
	}
	if overview.Environment != "preview" {
		t.Fatalf("overview environment = %q", overview.Environment)
	}
	for _, issue := range overview.Issues {
		if issue.Domain == workspacecore.IssueDomainEnv && issue.Reason == workspacecore.IssueReasonProfile {
			t.Fatalf("overview ignored preview binding: %#v", issue)
		}
	}
	assertRepositoryUnchanged(t, root, before)

	read := workspaceRequest(t, handler, http.MethodGet,
		"/api/workspace/profile-bindings/env?env=preview", nil)
	if read.Code != http.StatusOK || !strings.Contains(read.Body.String(), `"selectedProfile": "work"`) {
		t.Fatalf("GET status = %d; body = %s", read.Code, read.Body.String())
	}
	assertRepositoryUnchanged(t, root, before)

	recorder = workspaceRequest(t, handler, http.MethodPut,
		"/api/workspace/profile-bindings/env?env=preview", strings.NewReader(`{"profile":""}`))
	if recorder.Code != http.StatusOK {
		t.Fatalf("unbind status = %d; body = %s", recorder.Code, recorder.Body.String())
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &settings); err != nil {
		t.Fatal(err)
	}
	if settings.SelectedProfile != "" || settings.Profile == nil || settings.Profile.Source != "default" {
		t.Fatalf("unbind settings = %#v", settings)
	}
	assertRepositoryUnchanged(t, root, before)
}

func TestWorkspaceProfileGETSurfacesExistingStaleBindingUntilAutomaticUnbind(t *testing.T) {
	root := seedWorkspace(t)
	handler := newWorkspaceMux(t, root)
	seedDashboardProfiles(t)
	if _, err := profile.Upsert(profile.DomainEnv, "infisical", "fallback", profile.Profile{
		Backend: "infisical",
		Infisical: &profile.InfisicalProfile{
			SiteURL: "https://app.infisical.com",
			Credentials: &profile.InfisicalCredentials{
				ClientID: "fallback-client", ClientSecret: "fallback-secret",
			},
		},
	}, true); err != nil {
		t.Fatal(err)
	}
	before := snapshotRepositoryTree(t, root)

	bound := workspaceRequest(t, handler, http.MethodPut,
		"/api/workspace/profile-bindings/env?env=preview", strings.NewReader(`{"profile":"work"}`))
	if bound.Code != http.StatusOK {
		t.Fatalf("bind status = %d; body = %s", bound.Code, bound.Body.String())
	}

	// Simulate a stale selection already present from an older client/manual
	// edit. The current profile.Remove path purges these references itself.
	cfg, _, err := profile.Load()
	if err != nil {
		t.Fatal(err)
	}
	delete(cfg.EnvInfisical.Profiles, "work")
	if err := profile.Save(cfg); err != nil {
		t.Fatal(err)
	}

	read := workspaceRequest(t, handler, http.MethodGet,
		"/api/workspace/profile-bindings/env?env=preview", nil)
	if read.Code != http.StatusOK {
		t.Fatalf("GET status = %d; body = %s", read.Code, read.Body.String())
	}
	var settings workspaceProfileSettingsWire
	if err := json.Unmarshal(read.Body.Bytes(), &settings); err != nil {
		t.Fatal(err)
	}
	if settings.SelectedProfile != "work" || settings.Profile != nil {
		t.Fatalf("stale GET projection = %#v", settings)
	}
	assertRepositoryUnchanged(t, root, before)

	unbound := workspaceRequest(t, handler, http.MethodPut,
		"/api/workspace/profile-bindings/env?env=preview", strings.NewReader(`{"profile":""}`))
	if unbound.Code != http.StatusOK {
		t.Fatalf("unbind status = %d; body = %s", unbound.Code, unbound.Body.String())
	}
	if err := json.Unmarshal(unbound.Body.Bytes(), &settings); err != nil {
		t.Fatal(err)
	}
	if settings.SelectedProfile != "" || settings.Profile == nil ||
		settings.Profile.Name != "fallback" || settings.Profile.Source != "default" {
		t.Fatalf("automatic fallback = %#v", settings)
	}
	assertRepositoryUnchanged(t, root, before)
}

func TestConfigureDeleteRequiresAutomaticUnbindAndPreservesLocalFilesOnConflict(t *testing.T) {
	root := seedWorkspace(t)
	handler := newWorkspaceMux(t, root)
	seedDashboardProfiles(t)
	repositoryBefore := snapshotRepositoryTree(t, root)

	bound := workspaceRequest(t, handler, http.MethodPut,
		"/api/workspace/profile-bindings/env?env=preview", strings.NewReader(`{"profile":"work"}`))
	if bound.Code != http.StatusOK {
		t.Fatalf("bind status = %d; body = %s", bound.Code, bound.Body.String())
	}
	configPath, err := profile.ConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	credentialsPath, err := profile.CredentialsPath()
	if err != nil {
		t.Fatal(err)
	}
	bindingsPath, err := profile.BindingsPath()
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{configPath, credentialsPath, bindingsPath}
	beforeConflict := make(map[string][]byte, len(paths))
	for _, path := range paths {
		beforeConflict[path], err = os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
	}

	removed := workspaceRequest(t, handler, http.MethodDelete,
		"/api/configure/env/infisical/work", nil)
	if removed.Code != http.StatusConflict {
		t.Fatalf("bound delete status = %d; body = %s", removed.Code, removed.Body.String())
	}
	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(removed.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Error.Code != "PROFILE_IN_USE" {
		t.Fatalf("delete error code = %q", envelope.Error.Code)
	}
	for _, path := range paths {
		after, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if !bytes.Equal(after, beforeConflict[path]) {
			t.Fatalf("conflicting delete changed %s", path)
		}
	}
	assertRepositoryUnchanged(t, root, repositoryBefore)

	unbound := workspaceRequest(t, handler, http.MethodPut,
		"/api/workspace/profile-bindings/env?env=preview", strings.NewReader(`{"profile":""}`))
	if unbound.Code != http.StatusOK {
		t.Fatalf("unbind status = %d; body = %s", unbound.Code, unbound.Body.String())
	}
	removed = workspaceRequest(t, handler, http.MethodDelete,
		"/api/configure/env/infisical/work", nil)
	if removed.Code != http.StatusOK {
		t.Fatalf("unbound delete status = %d; body = %s", removed.Code, removed.Body.String())
	}
	assertRepositoryUnchanged(t, root, repositoryBefore)
}

func TestProjectProfileBindingSupportsAllManifestOwnedDomains(t *testing.T) {
	root := seedWorkspace(t)
	handler := newWorkspaceMux(t, root)
	seedDashboardProfiles(t)
	before := snapshotRepositoryTree(t, root)

	for _, domain := range []string{"env", "container", "deploy"} {
		recorder := workspaceRequest(t, handler, http.MethodPut,
			"/api/workspace/projects/web/profile-bindings/"+domain+"?env=preview",
			strings.NewReader(`{"profile":"work"}`))
		if recorder.Code != http.StatusOK {
			t.Fatalf("bind %s status = %d; body = %s", domain, recorder.Code, recorder.Body.String())
		}
		var settings workspaceSettingsWire
		if err := json.Unmarshal(recorder.Body.Bytes(), &settings); err != nil {
			t.Fatal(err)
		}
		if settings.Environment != "preview" || settings.Project.Name != "web" {
			t.Fatalf("bind %s settings = %#v", domain, settings)
		}
		var selected string
		switch domain {
		case "env":
			selected = settings.Project.Environment.SelectedProfile
		case "container":
			selected = settings.Project.Container.SelectedProfile
		case "deploy":
			selected = settings.Project.Deploy.SelectedProfile
		}
		if selected != "work" {
			t.Fatalf("bind %s selectedProfile = %q", domain, selected)
		}
		if strings.Contains(recorder.Body.String(), "must-not-leak") {
			t.Fatalf("bind %s leaked credentials: %s", domain, recorder.Body.String())
		}
		assertRepositoryUnchanged(t, root, before)

		recorder = workspaceRequest(t, handler, http.MethodPut,
			"/api/workspace/projects/web/profile-bindings/"+domain+"?env=preview",
			strings.NewReader(`{"profile":""}`))
		if recorder.Code != http.StatusOK {
			t.Fatalf("unbind %s status = %d; body = %s", domain, recorder.Code, recorder.Body.String())
		}
		assertRepositoryUnchanged(t, root, before)
	}
}

func TestProfileBindingPayloadAndDomainValidationNeverChangesRepository(t *testing.T) {
	for _, test := range []struct {
		name, path, body string
		want             int
	}{
		{name: "missing profile", path: "/api/workspace/profile-bindings/env?env=preview", body: `{}`, want: 400},
		{name: "extra field", path: "/api/workspace/profile-bindings/env?env=preview", body: `{"profile":"work","kind":"dotenv"}`, want: 400},
		{name: "trailing object", path: "/api/workspace/profile-bindings/env?env=preview", body: `{"profile":"work"} {}`, want: 400},
		{name: "unknown domain", path: "/api/workspace/projects/web/profile-bindings/ci?env=preview", body: `{"profile":"work"}`, want: 400},
		{name: "unknown profile", path: "/api/workspace/projects/web/profile-bindings/env?env=preview", body: `{"profile":"ghost"}`, want: 400},
		{name: "unsafe environment", path: "/api/workspace/projects/web/profile-bindings/env?env=../prod", body: `{"profile":"work"}`, want: 400},
		{name: "unknown project", path: "/api/workspace/projects/ghost/profile-bindings/env?env=preview", body: `{"profile":"work"}`, want: 404},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := seedWorkspace(t)
			handler := newWorkspaceMux(t, root)
			seedDashboardProfiles(t)
			before := snapshotRepositoryTree(t, root)
			recorder := workspaceRequest(t, handler, http.MethodPut, test.path, strings.NewReader(test.body))
			if recorder.Code != test.want {
				t.Fatalf("status = %d; want %d; body = %s", recorder.Code, test.want, recorder.Body.String())
			}
			assertRepositoryUnchanged(t, root, before)
		})
	}
}

func TestUnknownAndNonConfigurableManifestBackendsAreRejectedWithoutWriting(t *testing.T) {
	for _, backend := range []string{"vault", workspacecore.EnvBackendDotenv} {
		t.Run(backend, func(t *testing.T) {
			root := seedWorkspace(t)
			manifest, err := workspacecore.ReadManifest(root)
			if err != nil {
				t.Fatal(err)
			}
			manifest.Domains.Env.Kind = backend
			if err := workspacecore.WriteManifest(root, manifest); err != nil {
				t.Fatal(err)
			}
			handler := newWorkspaceMux(t, root)
			seedDashboardProfiles(t)
			before := snapshotRepositoryTree(t, root)
			recorder := workspaceRequest(t, handler, http.MethodPut,
				"/api/workspace/projects/web/profile-bindings/env?env=preview",
				strings.NewReader(`{"profile":"work"}`))
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d; body = %s", recorder.Code, recorder.Body.String())
			}
			assertRepositoryUnchanged(t, root, before)
		})
	}
}

func TestLegacyRepositoryMutationRoutesAlwaysReturnStableReadOnlyConflict(t *testing.T) {
	paths := []string{
		"/api/workspace/projects/web",
		"/api/workspace/projects/web/environment",
		"/api/workspace/projects/web/deploy",
		"/api/workspace/projects/web/container",
		"/api/workspace/projects/web/settings/deploy",
		"/api/workspace/projects/web/settings/container",
	}
	for _, path := range paths {
		for _, body := range []string{`{}`, `{this is not json`} {
			root := seedWorkspace(t)
			handler := newWorkspaceMux(t, root)
			before := snapshotRepositoryTree(t, root)
			recorder := workspaceRequest(t, handler, http.MethodPut, path, strings.NewReader(body))
			if recorder.Code != http.StatusConflict {
				t.Fatalf("%s status = %d; body = %s", path, recorder.Code, recorder.Body.String())
			}
			var envelope struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
				t.Fatal(err)
			}
			if envelope.Error.Code != "SERVE_REPOSITORY_READ_ONLY" {
				t.Fatalf("%s code = %q", path, envelope.Error.Code)
			}
			assertRepositoryUnchanged(t, root, before)
		}
	}
}

func TestManifestDraftRouteRequiresCurrentRevisionAndWritesAllowlistedFields(t *testing.T) {
	root := seedWorkspace(t)
	handler := newWorkspaceMux(t, root)

	read := workspaceRequest(t, handler, http.MethodGet, "/api/workspace/projects/web?env=dev", nil)
	if read.Code != http.StatusOK {
		t.Fatalf("GET status = %d; body = %s", read.Code, read.Body.String())
	}
	var settings struct {
		Revision string `json:"revision"`
	}
	if err := json.Unmarshal(read.Body.Bytes(), &settings); err != nil {
		t.Fatal(err)
	}
	if settings.Revision == "" {
		t.Fatal("project settings omitted manifest revision")
	}

	switched := workspaceRequest(t, handler, http.MethodPut,
		"/api/workspace/environment/backend?env=dev",
		strings.NewReader(fmt.Sprintf(`{"revision":%q,"backend":"dotenv"}`, settings.Revision)))
	if switched.Code != http.StatusOK {
		t.Fatalf("backend PUT status = %d; body = %s", switched.Code, switched.Body.String())
	}
	var switchedSettings workspaceProfileSettingsWire
	if err := json.Unmarshal(switched.Body.Bytes(), &switchedSettings); err != nil {
		t.Fatal(err)
	}
	if switchedSettings.Backend != "dotenv" || switchedSettings.Revision == settings.Revision {
		t.Fatalf("switched settings = %#v", switchedSettings)
	}

	body := fmt.Sprintf(`{
		"revision": %q,
		"changes": [{
			"project": "web",
			"general": {"buildVersion": "v2.0.0", "devCommand": "pnpm dev --host"},
			"environment": {"path": "/frontend", "inherits": false, "disabled": false}
		}]
	}`, switchedSettings.Revision)
	written := workspaceRequest(t, handler, http.MethodPut, "/api/workspace/manifest", strings.NewReader(body))
	if written.Code != http.StatusOK {
		t.Fatalf("PUT status = %d; body = %s", written.Code, written.Body.String())
	}
	manifest, err := workspacecore.ReadManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Projects[0].BuildVersion != "2.0.0" ||
		manifest.Projects[0].Domains.Dev.Command != "pnpm dev --host" ||
		manifest.Projects[0].Domains.Env.Path != "/frontend" {
		t.Fatalf("manifest = %#v", manifest.Projects[0])
	}
	if manifest.Domains == nil || manifest.Domains.Env == nil ||
		manifest.Domains.Env.Kind != "dotenv" {
		t.Fatalf("workspace env backend = %#v", manifest.Domains)
	}

	stale := workspaceRequest(t, handler, http.MethodPut, "/api/workspace/manifest", strings.NewReader(body))
	if stale.Code != http.StatusConflict || !strings.Contains(stale.Body.String(), "SERVE_MANIFEST_CONFLICT") {
		t.Fatalf("stale PUT status = %d; body = %s", stale.Code, stale.Body.String())
	}
}

func TestProfileBindingNoWorkspaceAndCrossOriginRemainClosed(t *testing.T) {
	handler := newWorkspaceMux(t, "")
	recorder := workspaceRequest(t, handler, http.MethodPut,
		"/api/workspace/profile-bindings/env?env=preview", strings.NewReader(`{"profile":"work"}`))
	if recorder.Code != http.StatusConflict {
		t.Fatalf("no workspace status = %d; body = %s", recorder.Code, recorder.Body.String())
	}

	root := seedWorkspace(t)
	handler = newWorkspaceMux(t, root)
	request := httptest.NewRequest(http.MethodPut,
		"/api/workspace/profile-bindings/env?env=preview",
		strings.NewReader(`{"profile":"work"}`))
	request.Host = workspaceTestHost
	request.Header.Set("Origin", "https://attacker.example.com")
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("cross-origin status = %d; body = %s", recorder.Code, recorder.Body.String())
	}
}
