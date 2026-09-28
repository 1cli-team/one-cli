package workspace

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/fsutil"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

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

// MarshalManifest renders the exact bytes WriteManifest publishes: canonical
// ordering, 2-space indentation, and a trailing newline.
func MarshalManifest(m *Manifest) ([]byte, error) {
	out := *m
	out.Version = ManifestVersion
	out.Projects = sortByRelativeDir(out.Projects)
	for i := range out.Projects {
		out.Projects[i].RelativeDir = ToPosixPath(out.Projects[i].RelativeDir)
		if out.Projects[i].Toolchain == "" {
			out.Projects[i].Toolchain = "node"
		}
		out.Projects[i].BuildVersion = NormalizeBuildVersion(out.Projects[i].BuildVersion)
	}

	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// WriteManifest persists the manifest to disk with 2-space indentation and
// a trailing newline (fs-extra parity). Publication is atomic: bytes are
// written to a sibling temporary file, synced, closed, and then renamed over
// the destination. Existing file permissions are preserved.
func WriteManifest(projectRoot string, m *Manifest) error {
	b, err := MarshalManifest(m)
	if err != nil {
		return err
	}
	return atomicWriteManifest(ManifestPath(projectRoot), b, 0o644)
}

type renameManifestFile func(string, string) error

func atomicWriteManifest(path string, data []byte, defaultMode fs.FileMode) error {
	return atomicWriteManifestWithRename(path, data, defaultMode, fsutil.ReplaceFile)
}

// atomicWriteManifestWithRename keeps publication injectable for focused
// failure-safety tests without process-global hooks. Production always passes
// os.Rename through atomicWriteManifest.
func atomicWriteManifestWithRename(
	path string,
	data []byte,
	defaultMode fs.FileMode,
	rename renameManifestFile,
) error {
	dir := filepath.Dir(path)
	mode := defaultMode.Perm()
	info, err := os.Stat(path)
	switch {
	case err == nil:
		mode = info.Mode().Perm()
	case !errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("stat manifest before atomic write: %w", err)
	}

	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+"-*.tmp")
	if err != nil {
		return fmt.Errorf("create manifest temp file: %w", err)
	}
	tmpPath := tmp.Name()
	closed := false
	defer func() {
		if !closed {
			_ = tmp.Close()
		}
		_ = os.Remove(tmpPath)
	}()

	if err := tmp.Chmod(mode); err != nil {
		return fmt.Errorf("set manifest temp permissions: %w", err)
	}
	written, err := tmp.Write(data)
	if err != nil {
		return fmt.Errorf("write manifest temp file: %w", err)
	}
	if written != len(data) {
		return fmt.Errorf("write manifest temp file: %w", io.ErrShortWrite)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync manifest temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close manifest temp file: %w", err)
	}
	closed = true
	if err := rename(tmpPath, path); err != nil {
		return fmt.Errorf("publish manifest atomically: %w", err)
	}
	return nil
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
		m.Env = nil
	} else {
		if init.Kind != EnvBackendInfisical {
			return i18n.Errorf("env.provider_invalid", init.Kind)
		}
		m.Env = &EnvironmentConfig{}
		if len(init.ConfigJSON) > 0 {
			if err := json.Unmarshal(init.ConfigJSON, m.Env); err != nil {
				return err
			}
		}
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

// UpdateProjectDev sets projects[].dev.command on the project
// entry keyed by relativeDir. Empty cmd clears the override block.
//
// Used by creation during `one add` to persist the derived
// dev command into the manifest, replacing the legacy Procfile.dev
// write path.
func UpdateProjectDev(projectRoot, relativeDir, cmd string) error {
	m, err := ReadManifest(projectRoot)
	if err != nil {
		return err
	}
	for i := range m.Projects {
		if m.Projects[i].RelativeDir == ToPosixPath(relativeDir) {
			m.Projects[i].Dev = nil
			if cmd != "" {
				m.Projects[i].Dev = &ProjectDevOverride{Command: cmd}
			}
			return WriteManifest(projectRoot, m)
		}
	}
	return nil
}

// RecordWorkspaceEnvKey appends key to the workspace-level env config's
// keys list (sorted, deduped, idempotent). Use this when a `one env set`
// runs at workspace-root scope — i.e. without -p and not inside any
// project. These keys are stored in m.Env.Keys and are usable by every project.
func RecordWorkspaceEnvKey(projectRoot, key string) error {
	if key == "" {
		return nil
	}
	m, err := EnsureManifest(projectRoot)
	if err != nil {
		return err
	}
	if m.Env == nil {
		m.Env = &EnvironmentConfig{}
	}
	for _, existing := range m.Env.Keys {
		if existing == key {
			return nil
		}
	}
	m.Env.Keys = append(m.Env.Keys, key)
	sort.Strings(m.Env.Keys)
	return WriteManifest(projectRoot, m)
}

// RecordProjectEnvKey appends key to projects[i].env.keys for the
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
		if m.Projects[i].Env == nil {
			m.Projects[i].Env = &ProjectEnvOverride{}
		}
		for _, existing := range m.Projects[i].Env.Keys {
			if existing == key {
				return nil
			}
		}
		m.Projects[i].Env.Keys = append(m.Projects[i].Env.Keys, key)
		sort.Strings(m.Projects[i].Env.Keys)
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

// SetManifestWorkspaceIdentity writes (or overwrites) the workspace
// identity (id / name). Used by `env init` to back-fill the field for
// workspaces created before the identity field existed; a fresh
// `one create` already sets it via the creation module. Both id and name are
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
