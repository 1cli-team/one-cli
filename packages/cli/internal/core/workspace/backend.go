package workspace

import (
	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
)

// Compatibility aliases for existing workspace callers.
const (
	EnvBackendInfisical = catalog.EnvInfisical
)

// EnvBackend returns infisical when the workspace has an env binding, otherwise empty.
func EnvBackend(m *Manifest) string {
	if m == nil || m.Env == nil {
		return ""
	}
	return EnvBackendInfisical
}

// SelectionForProject projects the environment source for template compatibility checks.
func SelectionForProject(m *Manifest, project *ManifestProject) map[string]string {
	out := map[string]string{}
	if m == nil {
		return out
	}
	if backend := EnvBackend(m); backend != "" {
		out["env"] = "env/" + backend
	}
	return out
}

// EnvironmentEnabled reports whether the workspace has an Infisical binding.
func EnvironmentEnabled(m *Manifest, relativeDir string) bool {
	return m != nil && m.Env != nil && m.Env.ProjectID != ""
}
