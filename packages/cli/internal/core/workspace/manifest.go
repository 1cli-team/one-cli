package workspace

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/fsutil"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

// ManifestFilename is the on-disk location of the workspace manifest at the
// project root.
const ManifestFilename = "one.manifest.toml"

// MiseConfigFilename is the workspace and project task configuration.
const MiseConfigFilename = "mise.toml"

// ManifestVersion is the current manifest schema generation.
const ManifestVersion = 2

// Manifest is the runtime projection. The on-disk TOML document uses keyed projects.
// Existing JSON projections retain their transport field names.
type Manifest struct {
	Version   int                `json:"version"`
	Workspace *ManifestWorkspace `json:"workspace,omitempty"`
	Env       *EnvironmentConfig `json:"env,omitempty"`
	Projects  []ManifestProject  `json:"projects"`
	source    []byte
}

type ManifestWorkspace struct {
	ID   string `json:"id" toml:"id"`
	Name string `json:"name" toml:"name"`
}

var DefaultEnvironments = []string{"dev", "staging", "prod"}

type EnvironmentConfig struct {
	SiteURL      string   `json:"siteUrl,omitempty" toml:"siteUrl,omitempty"`
	ProjectID    string   `json:"projectId" toml:"projectId"`
	Environments []string `json:"environments" toml:"environments"`
}

type ManifestProject struct {
	Name        string `json:"name"`
	RelativeDir string `json:"relativeDir"`
	Toolchain   string `json:"toolchain"`
}

type manifestDocument struct {
	Version   int                                `toml:"version"`
	Workspace *ManifestWorkspace                 `toml:"workspace"`
	Env       *manifestEnvironment               `toml:"env"`
	Projects  map[string]manifestProjectDocument `toml:"projects"`
}
type manifestEnvironment struct {
	Infisical *EnvironmentConfig `toml:"infisical"`
}
type manifestProjectDocument struct {
	Path      string `toml:"path"`
	Toolchain string `toml:"toolchain"`
}

// EnvironmentNames returns a copy; the default environment is always dev.
func EnvironmentNames(m *Manifest) []string {
	if m != nil && m.Env != nil && len(m.Env.Environments) > 0 {
		return append([]string(nil), m.Env.Environments...)
	}
	return append([]string(nil), DefaultEnvironments...)
}

// ParseManifest decodes only the current schema, including rejecting unknown fields.
func ParseManifest(raw []byte) (*Manifest, error) {
	var doc manifestDocument
	if err := toml.NewDecoder(bytes.NewReader(raw)).DisallowUnknownFields().Decode(&doc); err != nil {
		return nil, err
	}
	m := &Manifest{Version: doc.Version, Workspace: doc.Workspace, Projects: []ManifestProject{}, source: bytes.Clone(raw)}
	if doc.Env != nil {
		m.Env = doc.Env.Infisical
	}
	for name, p := range doc.Projects {
		m.Projects = append(m.Projects, ManifestProject{Name: name, RelativeDir: p.Path, Toolchain: p.Toolchain})
	}
	sort.Slice(m.Projects, func(i, j int) bool { return m.Projects[i].RelativeDir < m.Projects[j].RelativeDir })
	if err := ValidateManifest(m); err != nil {
		return nil, err
	}
	return m, nil
}

var environmentSlugPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

func ValidateManifest(m *Manifest) error {
	invalid := func(detail string) error { return cliErrors.New(cliErrors.MANIFEST_INVALID, detail) }
	if m.Version != ManifestVersion {
		return invalid(i18n.Tf("manifest.version_unsupported", m.Version, ManifestVersion))
	}
	if m.Workspace != nil && (strings.TrimSpace(m.Workspace.ID) == "" || strings.TrimSpace(m.Workspace.Name) == "") {
		return invalid(i18n.T("manifest.identity_required"))
	}
	if m.Env != nil {
		if strings.TrimSpace(m.Env.ProjectID) == "" {
			return invalid(i18n.T("infisical.binding_missing"))
		}
		hasDev := false
		seen := map[string]bool{}
		for _, env := range m.Env.Environments {
			if !environmentSlugPattern.MatchString(env) || len(env) > 128 || seen[env] {
				return invalid(i18n.Tf("manifest.environment_invalid", env))
			}
			seen[env] = true
			hasDev = hasDev || env == "dev"
		}
		if !hasDev {
			return invalid(i18n.T("manifest.dev_required"))
		}
	}
	names, paths := map[string]bool{}, map[string]bool{}
	for _, p := range m.Projects {
		path := p.RelativeDir
		if !IsValidProjectName(p.Name) || names[p.Name] {
			return invalid(i18n.Tf("manifest.project_invalid", p.Name))
		}
		names[p.Name] = true
		if path == "" || path == "." || strings.ContainsAny(path, "\\\x00") || strings.HasPrefix(path, "/") || strings.Contains(path, ":") || filepath.ToSlash(filepath.Clean(path)) != path || path == ".." || strings.HasPrefix(path, "../") || paths[path] {
			return invalid(i18n.Tf("manifest.path_invalid", p.Name, path))
		}
		paths[path] = true
		if p.Toolchain != "node" && p.Toolchain != "go" && p.Toolchain != "none" {
			return invalid(i18n.Tf("manifest.toolchain_invalid", p.Name, p.Toolchain))
		}
	}
	return nil
}

// ManifestPath returns the absolute path to one.manifest.toml under
// projectRoot.
func ManifestPath(projectRoot string) string {
	return filepath.Join(projectRoot, ManifestFilename)
}

// ResolveProjectRoot turns a possibly-empty -d flag value into an
// absolute workspace root. When dirFlag is empty the function walks up
// from cwd looking for one.manifest.toml; falling back to cwd if no
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

// ReadManifest loads and validates the manifest. Returns an empty manifest
// (no error) when the file does not exist. Only the current ManifestVersion
// and TOML format are accepted.
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
// an empty v2 manifest and an empty revision.
func ReadManifestSnapshot(projectRoot string) (*Manifest, string, error) {
	path := ManifestPath(projectRoot)
	raw, err := fsutil.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return emptyManifest(), "", nil
		}
		return nil, "", cliErrors.New(cliErrors.MANIFEST_INVALID, i18n.Tf("manifest.read_failed", path, err)).WithCause(err)
	}
	m, err := ParseManifest(raw)
	if err != nil {
		return nil, "", manifestParseError(path, err)
	}

	sum := sha256.Sum256(raw)
	return m, fmt.Sprintf("sha256:%x", sum[:]), nil
}

func emptyManifest() *Manifest {
	return &Manifest{
		Version:  ManifestVersion,
		Projects: []ManifestProject{},
	}
}

// Include the source location without dumping configuration contents.
func manifestParseError(path string, cause error) error {
	context := map[string]any{"path": path}
	detail := cause.Error()
	var missing *toml.StrictMissingError
	if errors.As(cause, &missing) {
		keys := []string{}
		for _, entry := range missing.Errors {
			keys = append(keys, strings.Join(entry.Key(), "."))
		}
		detail = i18n.Tf("manifest.unknown_fields", strings.Join(keys, ", "))
	}
	message := i18n.Tf("manifest.parse_detail", path, detail)
	var parse *toml.DecodeError
	if errors.As(cause, &parse) {
		line, column := parse.Position()
		context["line"], context["column"] = line, column
		message = i18n.Tf("manifest.parse_location", path, line, column, detail)
	}
	return cliErrors.New(cliErrors.MANIFEST_INVALID, message).WithContext(context).WithCause(cause)
}
