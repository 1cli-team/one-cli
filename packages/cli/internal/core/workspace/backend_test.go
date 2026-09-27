package workspace

// Locks the read/write helpers for the current deploy/container fields:
//
//   - projects[i].domains.container.namespace  (per-project)
//   - projects[i].domains.deploy.config.bucket (per-project, s3 only)
//   - manifest.domains.deploy.config.namespace (workspace, kustomize only)
//   - manifest.domains.deploy.config.kustomizationPath (workspace, kustomize only)

import (
	"encoding/json"
	"testing"
)

// rawConfig is a small helper that JSON-encodes a struct into a
// json.RawMessage suitable for stuffing into BackendRef.Config /
// ProjectDeployBackend.Config. Test-only; production sites use the typed
// per-kind config Encode helpers.
func rawConfig(t *testing.T, v any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("rawConfig: %v", err)
	}
	return raw
}
