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

// findProject returns a pointer into m.Projects matching projectName, or
// nil when not found. Internal helper for the per-project read accessors
// below — keeps each helper compact.
func findProject(m *Manifest, projectName string) *ManifestProject {
	if m == nil {
		return nil
	}
	for i := range m.Projects {
		if m.Projects[i].Name == projectName {
			return &m.Projects[i]
		}
	}
	return nil
}

// ProjectEnv returns the per-project env override, or nil when unset.
func ProjectEnv(m *Manifest, projectName string) *ProjectEnvOverride {
	d := findProject(m, projectName)
	if d == nil {
		return nil
	}
	return d.Env
}

// ProjectDev returns the dev command for projectName, or "" when there
// is no dev block or its Command is empty. Used by `one dev` to
// resolve the generated development adapter.
func ProjectDev(m *Manifest, projectName string) string {
	d := findProject(m, projectName)
	if d == nil || d.Dev == nil {
		return ""
	}
	return d.Dev.Command
}

// EnvironmentEnabled reports whether a command should fetch remote variables.
func EnvironmentEnabled(m *Manifest, relativeDir string) bool {
	if m == nil || m.Env == nil {
		return false
	}
	for _, project := range m.Projects {
		if project.RelativeDir == relativeDir {
			return project.Env == nil || !project.Env.Disabled
		}
	}
	return true
}
