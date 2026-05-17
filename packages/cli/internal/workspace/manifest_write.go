package workspace

import (
	"encoding/json"
	"os"
	"sort"
)

// ManifestProjectInput is the upsert payload from `add` and status fixes.
// PackageManager is optional and absent for Go projects (Go has no
// package manager concept here — go.mod is the single source of truth).
type ManifestProjectInput struct {
	Name           string
	RelativeDir    string
	TemplateID     string
	Toolchain      string
	PackageManager string
}

// EnsureManifest creates an empty manifest if missing and returns whatever
// is on disk.
func EnsureManifest(projectRoot string) (*Manifest, error) {
	if !HasManifest(projectRoot) {
		m := newEmptyManifestStub()
		if err := WriteManifest(projectRoot, m); err != nil {
			return nil, err
		}
		return m, nil
	}
	return ReadManifest(projectRoot)
}

// UpsertManifestProject adds or updates the manifest entry for the given
// project (keyed by relativeDir). Preserves the existing per-project
// `domains` override block (env / container / deploy).
func UpsertManifestProject(projectRoot string, input ManifestProjectInput) error {
	m, err := EnsureManifest(projectRoot)
	if err != nil {
		return err
	}
	relativeDir := ToPosixPath(input.RelativeDir)

	var preservedDomains *ProjectDomains
	buildVersion := DefaultBuildVersion
	kept := make([]ManifestProject, 0, len(m.Projects))
	for _, p := range m.Projects {
		if p.RelativeDir == relativeDir {
			preservedDomains = p.Domains
			buildVersion = NormalizeBuildVersion(p.BuildVersion)
			continue
		}
		kept = append(kept, p)
	}

	toolchain := input.Toolchain
	if toolchain == "" {
		toolchain = "node"
	}
	kept = append(kept, ManifestProject{
		Name:           input.Name,
		RelativeDir:    relativeDir,
		TemplateID:     input.TemplateID,
		Toolchain:      toolchain,
		BuildVersion:   buildVersion,
		PackageManager: input.PackageManager,
		Domains:        preservedDomains,
	})

	m.Projects = sortByRelativeDir(kept)
	return WriteManifest(projectRoot, m)
}

// WriteManifest persists the manifest to disk with 2-space indentation and
// a trailing newline (fs-extra parity). Preserves the top-level
// configuration fields verbatim — callers that want to mutate them
// should use the UpdateManifest* helpers below.
func WriteManifest(projectRoot string, m *Manifest) error {
	out := *m
	out.Version = ManifestVersion
	out.Projects = sortByRelativeDir(out.Projects)
	for i := range out.Projects {
		out.Projects[i].RelativeDir = ToPosixPath(out.Projects[i].RelativeDir)
		if out.Projects[i].Toolchain == "" {
			out.Projects[i].Toolchain = "node"
		}
		out.Projects[i].BuildVersion = NormalizeBuildVersion(out.Projects[i].BuildVersion)
		pruneProjectDomains(&out.Projects[i])
	}

	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(ManifestPath(projectRoot), b, 0o644)
}

// EnvInit is the atomic input for InitWorkspaceEnv: an env backend kind +
// opaque kind-specific config blob, plus optional workspace-level
// environment list updates. Used by `env init` when configuring a fresh
// secrets backend so the manifest's env block and environment list update
// in one write.
type EnvInit struct {
	Kind             string
	ConfigJSON       json.RawMessage
	EnvironmentNames []string
	DefaultEnv       string
}

// InitWorkspaceEnv writes the workspace-level env backend selection and
// (optionally) updates the workspace-level environments list. Replaces
// the legacy UpdateManifestEnv helper.
func InitWorkspaceEnv(projectRoot string, init EnvInit) error {
	m, err := EnsureManifest(projectRoot)
	if err != nil {
		return err
	}
	if init.Kind == "" {
		// Clear semantics: drop the env section entirely.
		if m.Domains != nil {
			m.Domains.Env = nil
		}
	} else {
		ensureWorkspaceEnv(m)
		m.Domains.Env.Kind = init.Kind
		m.Domains.Env.Config = init.ConfigJSON
	}
	if init.EnvironmentNames != nil {
		if m.Environments == nil {
			m.Environments = &Environments{}
		}
		m.Environments.Names = append([]string{}, init.EnvironmentNames...)
		if init.DefaultEnv != "" {
			m.Environments.Default = init.DefaultEnv
		} else if m.Environments.Default == "" && len(m.Environments.Names) > 0 {
			m.Environments.Default = m.Environments.Names[0]
		}
	} else if init.DefaultEnv != "" {
		if m.Environments == nil {
			m.Environments = &Environments{}
		}
		m.Environments.Default = init.DefaultEnv
	}
	return WriteManifest(projectRoot, m)
}

// SetWorkspaceEnvConfig updates only the workspace env backend's config
// blob, preserving Kind. Returns an error when no env backend
// has been selected yet (callers must call InitWorkspaceEnv first).
func SetWorkspaceEnvConfig(projectRoot string, configJSON json.RawMessage) error {
	m, err := EnsureManifest(projectRoot)
	if err != nil {
		return err
	}
	if m.Domains == nil || m.Domains.Env == nil {
		return ErrEnvBackendNotConfigured
	}
	m.Domains.Env.Config = configJSON
	return WriteManifest(projectRoot, m)
}

// ErrEnvBackendNotConfigured is returned by SetWorkspaceEnvConfig when
// the manifest has no env backend selected yet.
var ErrEnvBackendNotConfigured = newErrEnvBackendNotConfigured()

type envBackendNotConfiguredErr struct{}

func newErrEnvBackendNotConfigured() error       { return envBackendNotConfiguredErr{} }
func (envBackendNotConfiguredErr) Error() string { return "workspace env backend is not configured" }

// EnsureEnvironment guarantees that name is present in
// manifest.environments.names. Returns added=true when the environment
// list was modified (i.e. name was not already there). Idempotent —
// calling twice with the same name is a no-op on the second call.
func EnsureEnvironment(projectRoot, name string) (added bool, err error) {
	m, err := EnsureManifest(projectRoot)
	if err != nil {
		return false, err
	}
	if m.Environments == nil {
		m.Environments = &Environments{}
	}
	for _, existing := range m.Environments.Names {
		if existing == name {
			return false, nil
		}
	}
	m.Environments.Names = append(m.Environments.Names, name)
	if m.Environments.Default == "" {
		m.Environments.Default = name
	}
	return true, WriteManifest(projectRoot, m)
}

// UpdateProjectDev sets projects[].domains.dev.command on the project
// entry keyed by relativeDir. Empty cmd clears the override block.
// Mirrors UpdateManifestProjectEnv's read-modify-write pattern.
//
// Used by infra.SyncSubproject during `one add` to persist the derived
// dev command into the manifest, replacing the legacy Procfile.dev
// write path.
func UpdateProjectDev(projectRoot, relativeDir, cmd string) error {
	m, err := ReadManifest(projectRoot)
	if err != nil {
		return err
	}
	relativeDir = ToPosixPath(relativeDir)
	for i := range m.Projects {
		if m.Projects[i].RelativeDir != relativeDir {
			continue
		}
		if cmd == "" {
			if m.Projects[i].Domains != nil {
				m.Projects[i].Domains.Dev = nil
				pruneProjectDomains(&m.Projects[i])
			}
		} else {
			ensureProjectDomains(&m.Projects[i])
			m.Projects[i].Domains.Dev = &ProjectDevOverride{Command: cmd}
		}
		return WriteManifest(projectRoot, m)
	}
	return nil
}

// UpdateManifestProjectEnv sets the env override on a single project
// entry, keyed by relativeDir. Errors if the entry does not exist (use
// UpsertManifestProject first to create it).
func UpdateManifestProjectEnv(projectRoot, relativeDir string, env *ProjectEnvOverride) error {
	m, err := ReadManifest(projectRoot)
	if err != nil {
		return err
	}
	relativeDir = ToPosixPath(relativeDir)
	for i := range m.Projects {
		if m.Projects[i].RelativeDir == relativeDir {
			ensureProjectDomains(&m.Projects[i])
			m.Projects[i].Domains.Env = env
			pruneProjectDomains(&m.Projects[i])
			return WriteManifest(projectRoot, m)
		}
	}
	return nil
}

// RecordWorkspaceEnvKey appends key to the workspace-level env config's
// keys list (sorted, deduped, idempotent). Use this when a `one env set`
// runs at workspace-root scope — i.e. without -p and not inside any
// project. These keys are intended as project-global vars usable by every
// project. Stored inside m.Domains.Env.Config as a plain {"keys": [...]}
// blob alongside any backend-specific fields.
func RecordWorkspaceEnvKey(projectRoot, key string) error {
	if key == "" {
		return nil
	}
	m, err := EnsureManifest(projectRoot)
	if err != nil {
		return err
	}
	ensureWorkspaceEnv(m)
	cfg := map[string]json.RawMessage{}
	if len(m.Domains.Env.Config) > 0 {
		if err := json.Unmarshal(m.Domains.Env.Config, &cfg); err != nil {
			return err
		}
	}
	var keys []string
	if raw, ok := cfg["keys"]; ok {
		if err := json.Unmarshal(raw, &keys); err != nil {
			return err
		}
	}
	for _, existing := range keys {
		if existing == key {
			return nil
		}
	}
	keys = append(keys, key)
	sort.Strings(keys)
	keysRaw, err := json.Marshal(keys)
	if err != nil {
		return err
	}
	cfg["keys"] = keysRaw
	cfgRaw, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	m.Domains.Env.Config = cfgRaw
	return WriteManifest(projectRoot, m)
}

// WorkspaceEnvKeys returns the sorted union of variable names recorded at
// workspace-root scope, or nil when none. Reads the "keys" field embedded
// in m.Domains.Env.Config.
func WorkspaceEnvKeys(m *Manifest) []string {
	if m == nil || m.Domains == nil || m.Domains.Env == nil || len(m.Domains.Env.Config) == 0 {
		return nil
	}
	cfg := struct {
		Keys []string `json:"keys,omitempty"`
	}{}
	if err := json.Unmarshal(m.Domains.Env.Config, &cfg); err != nil {
		return nil
	}
	return cfg.Keys
}

// RecordProjectEnvKey appends key to projects[i].domains.env.keys for the
// named project. Sorted, deduped, idempotent — calling twice with the
// same key is a no-op on the second call. Caller passes the project's name
// (matches manifest.projects[i].name); unknown names are silently skipped
// so set semantics aren't blocked by a metadata bookkeeping concern.
func RecordProjectEnvKey(projectRoot, projectName, key string) error {
	if projectName == "" || key == "" {
		return nil
	}
	m, err := ReadManifest(projectRoot)
	if err != nil {
		return err
	}
	for i := range m.Projects {
		if m.Projects[i].Name != projectName {
			continue
		}
		ensureProjectDomains(&m.Projects[i])
		if m.Projects[i].Domains.Env == nil {
			m.Projects[i].Domains.Env = &ProjectEnvOverride{}
		}
		for _, existing := range m.Projects[i].Domains.Env.Keys {
			if existing == key {
				return nil
			}
		}
		m.Projects[i].Domains.Env.Keys = append(m.Projects[i].Domains.Env.Keys, key)
		sort.Strings(m.Projects[i].Domains.Env.Keys)
		return WriteManifest(projectRoot, m)
	}
	return nil
}

func sortByRelativeDir(in []ManifestProject) []ManifestProject {
	out := append([]ManifestProject{}, in...)
	sort.Slice(out, func(i, j int) bool {
		return out[i].RelativeDir < out[j].RelativeDir
	})
	return out
}

func newEmptyManifestStub() *Manifest {
	return &Manifest{
		Version:  ManifestVersion,
		Projects: []ManifestProject{},
	}
}

// RebuildManifest replaces the projects array wholesale with the supplied
// inputs, preserving each entry's per-project `domains` override block and
// every workspace-level field. Used by `add` to reconcile drift between
// the filesystem and the manifest.
func RebuildManifest(projectRoot string, inputs []ManifestProjectInput) (*Manifest, error) {
	current, err := ReadManifest(projectRoot)
	if err != nil {
		return nil, err
	}
	type prior struct {
		buildVersion string
		domains      *ProjectDomains
	}
	priorByDir := make(map[string]prior, len(current.Projects))
	for _, p := range current.Projects {
		priorByDir[p.RelativeDir] = prior{
			buildVersion: NormalizeBuildVersion(p.BuildVersion),
			domains:      p.Domains,
		}
	}

	projects := make([]ManifestProject, 0, len(inputs))
	for _, in := range inputs {
		rel := ToPosixPath(in.RelativeDir)
		toolchain := in.Toolchain
		if toolchain == "" {
			toolchain = "node"
		}
		p := priorByDir[rel]
		projects = append(projects, ManifestProject{
			Name:           in.Name,
			RelativeDir:    rel,
			TemplateID:     in.TemplateID,
			Toolchain:      toolchain,
			BuildVersion:   NormalizeBuildVersion(p.buildVersion),
			PackageManager: in.PackageManager,
			Domains:        p.domains,
		})
	}
	m := &Manifest{
		Version:      ManifestVersion,
		Workspace:    current.Workspace,
		Environments: current.Environments,
		Domains:      current.Domains,
		Projects:     sortByRelativeDir(projects),
	}
	if err := WriteManifest(projectRoot, m); err != nil {
		return nil, err
	}
	return m, nil
}

// SetManifestWorkspaceIdentity writes (or overwrites) the workspace
// identity (id / name). Used by `env init` to back-fill the field for
// workspaces created before the identity field existed; a fresh
// `one create` already sets it via scaffold. Both id and name are
// required.
func SetManifestWorkspaceIdentity(projectRoot, id, name string) error {
	m, err := EnsureManifest(projectRoot)
	if err != nil {
		return err
	}
	if m.Workspace == nil {
		m.Workspace = &ManifestWorkspace{}
	}
	m.Workspace.ID = id
	m.Workspace.Name = name
	return WriteManifest(projectRoot, m)
}
