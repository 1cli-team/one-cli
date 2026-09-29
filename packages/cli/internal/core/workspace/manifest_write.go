package workspace

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/configedit"
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

// MarshalManifest renders TOML, preserving existing comments and unrelated formatting.
// Fresh manifests contain only configuration, with no generated comments.
func MarshalManifest(m *Manifest) ([]byte, error) {
	copy := *m
	copy.Version = ManifestVersion
	m = &copy
	if err := ValidateManifest(m); err != nil {
		return nil, err
	}
	var out bytes.Buffer
	fmt.Fprintf(&out, "version = %d\n", ManifestVersion)
	table := func(header string, value any) error {
		raw, err := toml.Marshal(value)
		if err != nil {
			return err
		}
		fmt.Fprintf(&out, "\n[%s]\n", header)
		out.Write(raw)
		return nil
	}
	if m.Workspace != nil {
		if err := table("workspace", m.Workspace); err != nil {
			return nil, err
		}
	}
	if m.Env != nil {
		if err := table("env.infisical", m.Env); err != nil {
			return nil, err
		}
	}
	for _, p := range sortByRelativeDir(m.Projects) {
		key, _ := toml.Marshal(map[string]int{p.Name: 0})
		name := strings.TrimSpace(strings.SplitN(string(key), " = ", 2)[0])
		if err := table("projects."+name, manifestProjectDocument{Path: p.RelativeDir, Toolchain: p.Toolchain}); err != nil {
			return nil, err
		}
	}
	if len(m.source) > 0 {
		return configedit.UpdateTOML(m.source, out.Bytes())
	}
	return out.Bytes(), nil
}

// WriteManifest persists the TOML manifest. Publication is atomic: bytes are
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
}

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
		if init.EnvironmentNames != nil {
			m.Env.Environments = append([]string(nil), init.EnvironmentNames...)
		}
		if len(m.Env.Environments) == 0 {
			m.Env.Environments = append([]string(nil), DefaultEnvironments...)
		}
	}
	return WriteManifest(projectRoot, m)
}

func EnsureEnvironment(projectRoot, name string) (bool, error) {
	m, err := ReadManifest(projectRoot)
	if err != nil {
		return false, err
	}
	if m.Env == nil {
		return false, i18n.Errorf("infisical.binding_missing")
	}
	for _, env := range m.Env.Environments {
		if env == name {
			return false, nil
		}
	}
	m.Env.Environments = append(m.Env.Environments, name)
	return true, WriteManifest(projectRoot, m)
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
