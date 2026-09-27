package workspace

import (
	"encoding/json"
	"strings"

	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
)

// backend.go is the read-side projection for "what backend has this workspace
// selected for each domain". Identity remains owned by core/backend; the
// aliases below preserve existing workspace callers without another literal
// vocabulary.
//
// All helpers read the current manifest fields (Manifest.Domains, ManifestProject.Domains).

// Compatibility aliases for existing workspace callers.
const (
	EnvBackendDotenv    = catalog.EnvDotenv
	EnvBackendInfisical = catalog.EnvInfisical
)

// EnvBackend returns the bare backend name selected for the workspace's
// env domain ("dotenv" / "infisical"), or "" if unset.
func EnvBackend(m *Manifest) string {
	if m == nil || m.Domains == nil || m.Domains.Env == nil {
		return ""
	}
	return m.Domains.Env.Kind
}

// EnvConfigRaw returns the raw JSON of the workspace env backend's
// kind-specific config, suitable for unmarshalling into a typed config
// struct (DotenvConfig / InfisicalConfig). Returns nil when no config has
// been written.
func EnvConfigRaw(m *Manifest) json.RawMessage {
	if m == nil || m.Domains == nil || m.Domains.Env == nil {
		return nil
	}
	return m.Domains.Env.Config
}

// WorkspaceID returns the workspace identity id, or "" when older
// manifests have not been back-filled yet.
func WorkspaceID(m *Manifest) string {
	if m == nil || m.Workspace == nil {
		return ""
	}
	return strings.TrimSpace(m.Workspace.ID)
}

// SelectionForProject collapses the workspace-level domain selections and
// any per-project container / deploy overrides into a single map keyed by
// domain ("container" / "deploy" / "env"). Empty values are dropped so
// callers can range without nil-checking. Used by infra.SyncProject to
// know which backend to run per domain. CI is not part of this map and is
// not generated implicitly; dev commands live on each project record.
//
// Values are namespaced ids ("env/dotenv", "deploy/kustomize", ...) for
// compatibility with the existing infra dispatch. The dispatch strips the
// prefix before switching on the bare name.
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

func projectDomains(m *Manifest, projectName string) *ProjectDomains {
	p := findProject(m, projectName)
	if p == nil {
		return nil
	}
	return p.Domains
}

// ProjectEnv returns the per-project env override, or nil when unset.
// Exported because secrets backends (dotenv path resolution, infisical
// disabled-flag check) read it directly.
func ProjectEnv(m *Manifest, projectName string) *ProjectEnvOverride {
	d := projectDomains(m, projectName)
	if d == nil {
		return nil
	}
	return d.Env
}

// ProjectDev returns the dev command for projectName, or "" when there
// is no domains.dev block or its Command is empty. Used by `one dev` to
// build its supervisor entry list.
func ProjectDev(m *Manifest, projectName string) string {
	d := projectDomains(m, projectName)
	if d == nil || d.Dev == nil {
		return ""
	}
	return d.Dev.Command
}
