package workspace

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/fsutil"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

// ManifestFilename is the on-disk location of the workspace manifest at the
// project root.
const ManifestFilename = "one.manifest.json"

// MiseConfigFilename is the managed fragment marking a mise-enabled workspace.
const MiseConfigFilename = ".mise/conf.d/one.toml"

// ManifestVersion is the current manifest schema generation.
const ManifestVersion = 1

// Manifest is the parsed one.manifest.json document.
//
// Current layout:
//   - workspace: identity only (id, name)
//   - environments: environment-name list and default for secrets backends
//   - domains: workspace environment backend and its config
//   - projects[]: each project carries identity (name, relativeDir,
//     templateId, toolchain, buildVersion, packageManager) plus an optional
//     domains block with environment overrides and a development command.
type Manifest struct {
	Version      int                `json:"version"`
	Workspace    *ManifestWorkspace `json:"workspace,omitempty"`
	Environments *Environments      `json:"environments,omitempty"`
	Domains      *WorkspaceDomains  `json:"domains,omitempty"`
	Projects     []ManifestProject  `json:"projects"`
}

// ManifestWorkspace describes the workspace identity. The shared manifest no longer carries
// roots (apps/services/packages are hard-wired in roots.go) or the
// packageManager (which was never read at workspace scope; the field still
// exists per-project on ManifestProject).
type ManifestWorkspace struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Environments names the dotenv files or Infisical environments available
// to the workspace ("dev" / "preview" / "prod" by default).
//
// Default is the env name used when --env is omitted; it must appear in
// Names. New workspaces seed `["dev","preview","prod"]` with default "dev".
type Environments struct {
	Names   []string `json:"names,omitempty"`
	Default string   `json:"default,omitempty"`
}

// DefaultEnvironments is the canonical environment list stamped into a
// fresh manifest.environments.names. Same value is mirrored by the
// infisical package; defining it here keeps the workspace layer
// independent of any specific secrets backend.
var DefaultEnvironments = []string{"dev", "preview", "prod"}

// WorkspaceDomains selects the optional workspace environment backend.
type WorkspaceDomains struct {
	Env *BackendRef `json:"env,omitempty"`
}

// BackendRef is the workspace-level "selected backend" for a single domain.
// `Kind` is the bare backend name ("infisical" or "dotenv").
// `Config` is decoded via the environment backend's typed accessors.
type BackendRef struct {
	Kind   string          `json:"kind,omitempty"`
	Config json.RawMessage `json:"config,omitempty"`
}

// ManifestProject is one project entry in manifest.projects[]. Identity
// fields are flat at the top; backend overrides live inside the optional
// `domains` block (mirroring the workspace shape).
type ManifestProject struct {
	Name           string          `json:"name"`
	RelativeDir    string          `json:"relativeDir"`
	TemplateID     string          `json:"templateId"`
	Toolchain      string          `json:"toolchain"`
	BuildVersion   string          `json:"buildVersion"`
	PackageManager string          `json:"packageManager,omitempty"`
	Domains        *ProjectDomains `json:"domains,omitempty"`
}

// ProjectDomains holds environment overrides and the development command.
// The environment backend is always inherited from the workspace.
type ProjectDomains struct {
	Env *ProjectEnvOverride `json:"env,omitempty"`
	Dev *ProjectDevOverride `json:"dev,omitempty"`
}

// ProjectDevOverride is the per-project dev command for `one dev`.
// Written by `one add` at scaffold time (derived from package.json
// scripts + toolchain). Users can hand-edit Command in the manifest to
// customise — there's no auto-sync if package.json scripts change after
// scaffold; manifest is the source of truth.
//
// Empty Command (or missing block) means "this project is not part of
// `one dev`" — the supervisor will skip it.
type ProjectDevOverride struct {
	// Command is the full shell line executed by the platform shell
	// (sh on Unix, cmd.exe on Windows).
	Command string `json:"command,omitempty"`
}

// ProjectEnvOverride is the per-project env override. Carries no `kind`
// field because secrets backends are workspace-scoped.
//
// Keys is the sorted union of variable names ever set against this project
// (across every environment). Writing here on every `one env set` lets
// `one env check` lint every declared environment for completeness — i.e.
// catch the "added FOO to dev, forgot prod" case before deploy. Values
// themselves never live in the manifest; only names.
type ProjectEnvOverride struct {
	Path     string   `json:"path,omitempty"`
	Inherits *bool    `json:"inherits,omitempty"`
	Disabled bool     `json:"disabled,omitempty"`
	Keys     []string `json:"keys,omitempty"`
}

// ManifestPath returns the absolute path to one.manifest.json under
// projectRoot.
func ManifestPath(projectRoot string) string {
	return filepath.Join(projectRoot, ManifestFilename)
}

// ResolveProjectRoot turns a possibly-empty -d flag value into an
// absolute workspace root. When dirFlag is empty the function walks up
// from cwd looking for one.manifest.json; falling back to cwd if no
// workspace marker is found. Used by per-domain CLI commands so each
// verb's RunE can resolve its working root the same way.
func ResolveProjectRoot(dirFlag string) (string, error) {
	if dirFlag == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		root, err := WalkUpToManifest(cwd)
		if err == nil && root != "" {
			return root, nil
		}
		return cwd, nil
	}
	if filepath.IsAbs(dirFlag) {
		return dirFlag, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(cwd, dirFlag), nil
}

// HasManifest reports whether the manifest file exists.
func HasManifest(projectRoot string) bool {
	_, err := os.Stat(ManifestPath(projectRoot))
	return err == nil
}

// IsOneProjectRoot is an alias for HasManifest.
func IsOneProjectRoot(projectRoot string) bool {
	return HasManifest(projectRoot)
}

// ReadManifest loads and validates the manifest. Returns an empty manifest
// (no error) when the file does not exist. Only the current ManifestVersion
// is accepted; older manifests must be migrated by hand (see CHANGELOG).
func ReadManifest(projectRoot string) (*Manifest, error) {
	manifest, _, err := ReadManifestSnapshot(projectRoot)
	return manifest, err
}

// ReadManifestSnapshot loads the manifest together with a revision derived
// from its exact on-disk bytes. Dashboard edits send this revision back when
// they publish a draft so a stale browser tab cannot overwrite changes made
// by another CLI process or editor in the meantime.
//
// A missing manifest keeps the historical ReadManifest behaviour: it returns
// an empty v1 manifest and an empty revision.
func ReadManifestSnapshot(projectRoot string) (*Manifest, string, error) {
	path := ManifestPath(projectRoot)
	raw, err := fsutil.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return emptyManifest(), "", nil
		}
		return nil, "", cliErrors.New(cliErrors.MANIFEST_INVALID, i18n.T("manifest.parse_failed"))
	}
	var m Manifest
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		if err.Error() == `json: unknown field "deploy"` || err.Error() == `json: unknown field "container"` {
			return nil, "", cliErrors.New(cliErrors.MANIFEST_INVALID, i18n.T("manifest.retired_fields"))
		}
		return nil, "", cliErrors.New(cliErrors.MANIFEST_INVALID, i18n.T("manifest.parse_failed"))
	}
	if m.Version != ManifestVersion {
		msg := i18n.Tf("manifest.version_unsupported", m.Version, ManifestVersion)
		if m.Version > ManifestVersion {
			msg += i18n.T("manifest.upgrade_hint")
		}
		if m.Version == 0 {
			msg += i18n.T("manifest.migration_hint")
		}
		return nil, "", cliErrors.New(cliErrors.MANIFEST_INVALID, msg)
	}
	for i := range m.Projects {
		m.Projects[i].RelativeDir = ToPosixPath(m.Projects[i].RelativeDir)
		if m.Projects[i].Toolchain == "" {
			m.Projects[i].Toolchain = "node"
		}
	}
	sum := sha256.Sum256(raw)
	return &m, fmt.Sprintf("sha256:%x", sum[:]), nil
}

func emptyManifest() *Manifest {
	return &Manifest{
		Version:  ManifestVersion,
		Projects: []ManifestProject{},
	}
}
