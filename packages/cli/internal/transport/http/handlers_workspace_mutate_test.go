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

	workspacecore "github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
)

const workspaceTestHost = "dashboard.test"

func seedWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	manifest := &workspacecore.Manifest{
		Version:      workspacecore.ManifestVersion,
		Workspace:    &workspacecore.ManifestWorkspace{ID: "demo", Name: "demo"},
		Environments: &workspacecore.Environments{Names: []string{"dev", "staging", "prod"}, Default: "dev"},
		Domains: &workspacecore.WorkspaceDomains{
			Env: &workspacecore.BackendRef{Kind: workspacecore.EnvBackendInfisical},
		},
		Projects: []workspacecore.ManifestProject{{
			Name: "web", RelativeDir: "apps/web", TemplateID: "react-spa", Toolchain: "node",
			Domains: &workspacecore.ProjectDomains{},
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
	} `json:"project"`
}

func TestLegacyRepositoryMutationRoutesAlwaysReturnStableReadOnlyConflict(t *testing.T) {
	paths := []string{
		"/api/workspace/projects/web",
		"/api/workspace/projects/web/environment",
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
