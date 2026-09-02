package serve

import (
	"net/http"
	"strings"
	"testing"

	workspacecore "github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
)

func TestSecretRoutesRejectDotenvBackend(t *testing.T) {
	root := seedWorkspace(t)
	manifest, err := workspacecore.ReadManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	manifest.Domains.Env.Kind = workspacecore.EnvBackendDotenv
	if err := workspacecore.WriteManifest(root, manifest); err != nil {
		t.Fatal(err)
	}
	handler := newWorkspaceMux(t, root)
	before := snapshotRepositoryTree(t, root)

	for _, tc := range []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "list", method: http.MethodGet, path: "/api/workspace/secrets"},
		{name: "get", method: http.MethodGet, path: "/api/workspace/secrets/HELLO"},
		{name: "create", method: http.MethodPost, path: "/api/workspace/secrets", body: `{"key":"HELLO","value":"world"}`},
		{name: "update", method: http.MethodPut, path: "/api/workspace/secrets/HELLO", body: `{"value":"world"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := workspaceRequest(t, handler, tc.method, tc.path, strings.NewReader(tc.body))
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("%s status = %d; body = %s", tc.name, recorder.Code, recorder.Body.String())
			}
			if !strings.Contains(recorder.Body.String(), `"code":"ENV_BACKEND_INVALID"`) {
				t.Fatalf("%s body = %s", tc.name, recorder.Body.String())
			}
		})
	}

	assertRepositoryUnchanged(t, root, before)
}
