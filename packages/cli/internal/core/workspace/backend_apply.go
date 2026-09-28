package workspace

import "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"

// SetProjectBuildVersion writes projects[name].buildVersion. Versions are
// stored without a leading "v" even when Docker tags use one.
func SetProjectBuildVersion(projectRoot, projectName, version string) error {
	m, err := EnsureManifest(projectRoot)
	if err != nil {
		return err
	}
	idx, err := projectIndex(m, projectName)
	if err != nil {
		return err
	}
	m.Projects[idx].BuildVersion = NormalizeBuildVersion(version)
	return WriteManifest(projectRoot, m)
}

// projectIndex returns the index of projectName in m.Projects, or an
// error if not found.
func projectIndex(m *Manifest, projectName string) (int, error) {
	for i := range m.Projects {
		if m.Projects[i].Name == projectName {
			return i, nil
		}
	}
	return -1, i18n.Errorf("workspace.project_missing", projectName)
}

func ensureWorkspaceEnv(m *Manifest) {
	if m.Env == nil {
		m.Env = &EnvironmentConfig{}
	}
}
