// Selection write helpers shared by `one create --secrets` (via
// ApplyBackendSelection) and the template default-application flow in
// `one add` (via SetWorkspaceSelection / SetPerProjectSelection).
//
// All helpers operate on the current manifest shape (Manifest.Domains,
// ManifestProject.Domains).
package workspace

import (
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

// ApplyBackendSelection writes a list of fully-qualified ids
// ("<domain>/<backend>") into the manifest domains block. Selecting two ids for
// the same domain is rejected (mutual exclusion).
func ApplyBackendSelection(projectRoot string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	m, err := EnsureManifest(projectRoot)
	if err != nil {
		return err
	}
	seen := map[string]string{}
	for _, raw := range ids {
		idx := strings.IndexByte(raw, '/')
		if idx <= 0 || idx == len(raw)-1 {
			return i18n.Errorf("backend.id_invalid", raw)
		}
		domain := raw[:idx]
		if prev, dupe := seen[domain]; dupe && prev != raw {
			return i18n.Errorf("backend.selection_conflict", domain, prev, raw)
		}
		seen[domain] = raw
		applyDomainSelection(m, domain, raw)
	}
	return WriteManifest(projectRoot, m)
}

// SetWorkspaceSelection writes one workspace-scoped selection (env) into
// the manifest domains block. Returns the previous bare-name backend (empty if
// unset). domain is the bare domain name. (ci / dev are not persisted.)
func SetWorkspaceSelection(projectRoot, domain, id string) (previous string, err error) {
	m, err := EnsureManifest(projectRoot)
	if err != nil {
		return "", err
	}
	previous = previousWorkspaceSelection(m, domain)
	applyDomainSelection(m, domain, id)
	if err := WriteManifest(projectRoot, m); err != nil {
		return "", err
	}
	return previous, nil
}

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

// applyDomainSelection writes the given namespaced id into the appropriate
// field on m. Unknown domains are silently ignored — the registry
// validation test catches mismatches at build time. CI / Dev are not
// persisted; selections targeting those domains are silently dropped.
func applyDomainSelection(m *Manifest, domain, id string) {
	bare := stripDomainPrefix(id)
	switch domain {
	case "env":
		ensureWorkspaceEnv(m)
		m.Domains.Env.Kind = bare
		// Seed the workspace-level environment list on first selection.
		// Both backends (dotenv / infisical) treat
		// manifest.environments.names as the authoritative list, so
		// stamping defaults here keeps `one env get/set --env <name>`
		// usable from the moment the workspace is created.
		if m.Environments == nil {
			m.Environments = &Environments{}
		}
		if len(m.Environments.Names) == 0 {
			m.Environments.Names = append([]string{}, DefaultEnvironments...)
		}
		if m.Environments.Default == "" && len(m.Environments.Names) > 0 {
			m.Environments.Default = m.Environments.Names[0]
		}
	}
}

// previousWorkspaceSelection returns the prior bare-name backend for a
// workspace-scoped domain, used so callers can roll back on failure. CI /
// Dev domains always report empty because they are not persisted as backend selections.
func previousWorkspaceSelection(m *Manifest, domain string) string {
	switch domain {
	case "env":
		if m != nil && m.Domains != nil && m.Domains.Env != nil {
			return m.Domains.Env.Kind
		}
	}
	return ""
}

// stripDomainPrefix turns "env/dotenv" / "deploy/aws-s3" / "container/docker"
// into the bare backend name. Inputs without a slash pass through.
func stripDomainPrefix(id string) string {
	for i := 0; i < len(id); i++ {
		if id[i] == '/' {
			if i == len(id)-1 {
				return id
			}
			return id[i+1:]
		}
	}
	return id
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

func ensureProjectDomains(p *ManifestProject) {
	if p.Domains == nil {
		p.Domains = &ProjectDomains{}
	}
}

// pruneProjectDomains drops the Domains pointer when every override is
// empty, keeping JSON output tidy.
func pruneProjectDomains(p *ManifestProject) {
	if p.Domains == nil {
		return
	}
	if p.Domains.Env == nil && p.Domains.Dev == nil {
		p.Domains = nil
	}
}

func ensureWorkspaceEnv(m *Manifest) {
	if m.Domains == nil {
		m.Domains = &WorkspaceDomains{}
	}
	if m.Domains.Env == nil {
		m.Domains.Env = &BackendRef{}
	}
}
