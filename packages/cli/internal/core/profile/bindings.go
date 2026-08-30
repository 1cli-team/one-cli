package profile

// bindings.go owns the Dashboard's environment-aware profile selections.
//
// Profile definitions and credentials continue to live in config.json and
// credentials.json.  This third, machine-local file only records which named
// profile a workspace/environment (and, optionally, one of its projects)
// selects:
//
//   ~/.config/one/profile-bindings.json
//
// The canonical workspace root is the identity key.  That deliberately keeps
// two checkouts/copies of a workspace independent even when their shared
// manifests carry the same workspace id.  Nothing in this store is written to
// the workspace itself.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/gofrs/flock"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
)

const (
	bindingsSchemaVersion  = 1
	bindingsLockRetryDelay = 10 * time.Millisecond
	bindingsLockTimeout    = 2 * time.Second
)

var (
	bindingsMu          sync.RWMutex
	bindingIdentifierRE = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)
)

// bindingsFile is intentionally separate from Config.  Config is the v1
// profile-definition schema and remains byte-for-byte compatible with older
// clients; environment-aware Dashboard selections never cause config.json or
// credentials.json to be rewritten.
type bindingsFile struct {
	Version    int                          `json:"version"`
	Workspaces map[string]bindingsWorkspace `json:"workspaces,omitempty"`
}

type bindingsWorkspace struct {
	ID           string                         `json:"id,omitempty"`
	Name         string                         `json:"name,omitempty"`
	Environments map[string]bindingsEnvironment `json:"environments,omitempty"`
}

type bindingsEnvironment struct {
	Profiles map[string]string                 `json:"profiles,omitempty"`
	Projects map[string]bindingsProjectProfile `json:"projects,omitempty"`
}

type bindingsProjectProfile struct {
	Profiles map[string]string `json:"profiles,omitempty"`
}

// BindingsPath returns the machine-local environment profile-selection file.
func BindingsPath() (string, error) {
	root, err := configRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "profile-bindings.json"), nil
}

// EnvironmentProfileBinding returns the raw binding at exactly one scope.
// Unlike Resolve, this read does not require the referenced Profile definition
// to exist. Dashboard projections use that distinction to surface and remove
// stale machine-local selections instead of silently hiding them.
//
// An empty projectName reads the Workspace binding; a non-empty projectName
// reads only that Project's direct binding and does not fall back to Workspace.
func EnvironmentProfileBinding(
	root, projectName, environment string,
	domain Domain,
	backend string,
) (string, error) {
	if err := validateBackend(domain, backend); err != nil {
		return "", err
	}
	validatedProjectName, err := validateBindingMetadata("project name", projectName, true)
	if err != nil {
		return "", err
	}
	projectBinding, workspaceBinding, err := environmentBindingNamesAt(
		root, environment, validatedProjectName, SectionKey(domain, backend),
	)
	if err != nil {
		return "", err
	}
	if validatedProjectName != "" {
		return projectBinding, nil
	}
	return workspaceBinding, nil
}

// BindEnvironmentProfile records one environment-aware workspace or project
// selection.  The selected profile must already exist in the matching typed
// profile section.  Validation reads config.json/credentials.json but this
// operation writes only profile-bindings.json.
func BindEnvironmentProfile(
	workspaceID, workspaceName, root, projectName, environment string,
	domain Domain,
	backend, name string,
) error {
	canonicalRoot, err := canonicalBindingRoot(root)
	if err != nil {
		return err
	}
	environment, err = validateBindingEnvironment(environment)
	if err != nil {
		return err
	}
	name, err = validateBindingProfileName(name)
	if err != nil {
		return err
	}
	if err := validateBackend(domain, backend); err != nil {
		return err
	}

	workspaceID, err = validateBindingMetadata("workspace id", workspaceID, false)
	if err != nil {
		return err
	}
	workspaceName, err = validateBindingMetadata("workspace name", workspaceName, false)
	if err != nil {
		return err
	}
	projectName, err = validateBindingMetadata("project name", projectName, true)
	if err != nil {
		return err
	}

	bindingsMu.Lock()
	defer bindingsMu.Unlock()

	path, err := BindingsPath()
	if err != nil {
		return err
	}
	return updateBindingsAt(context.Background(), path, func(bindings *bindingsFile) (bool, error) {
		// Re-check existence while holding the binding store's exclusive lock.
		// Profile removal holds the matching shared lock through its config Save,
		// so a writer queued behind removal cannot publish a newly stale binding.
		cfg, _, err := Load()
		if err != nil {
			return false, err
		}
		if exists, names := profileExists(cfg, domain, backend, name); !exists {
			source := "workspace-environment"
			if projectName != "" {
				source = "workspace-project-environment"
			}
			return false, profileNotFound(SectionKey(domain, backend), name, source, names)
		}
		if bindings.Workspaces == nil {
			bindings.Workspaces = make(map[string]bindingsWorkspace)
		}
		workspace := bindings.Workspaces[canonicalRoot]
		if workspaceID != "" {
			workspace.ID = workspaceID
		}
		if workspaceName != "" {
			workspace.Name = workspaceName
		}
		if workspace.Environments == nil {
			workspace.Environments = make(map[string]bindingsEnvironment)
		}
		selection := workspace.Environments[environment]
		sectionKey := SectionKey(domain, backend)
		if projectName == "" {
			if selection.Profiles == nil {
				selection.Profiles = make(map[string]string)
			}
			selection.Profiles[sectionKey] = name
		} else {
			if selection.Projects == nil {
				selection.Projects = make(map[string]bindingsProjectProfile)
			}
			project := selection.Projects[projectName]
			if project.Profiles == nil {
				project.Profiles = make(map[string]string)
			}
			project.Profiles[sectionKey] = name
			selection.Projects[projectName] = project
		}
		workspace.Environments[environment] = selection
		bindings.Workspaces[canonicalRoot] = workspace
		return true, nil
	})
}

// UnbindEnvironmentProfile removes only the selected environment-aware
// binding.  It is idempotent and prunes empty project, environment, and
// workspace objects.  Profile definitions and legacy bindings are untouched.
func UnbindEnvironmentProfile(
	root, projectName, environment string,
	domain Domain,
	backend string,
) error {
	canonicalRoot, err := canonicalBindingRoot(root)
	if err != nil {
		return err
	}
	environment, err = validateBindingEnvironment(environment)
	if err != nil {
		return err
	}
	projectName, err = validateBindingMetadata("project name", projectName, true)
	if err != nil {
		return err
	}
	if err := validateBackend(domain, backend); err != nil {
		return err
	}

	bindingsMu.Lock()
	defer bindingsMu.Unlock()

	path, err := BindingsPath()
	if err != nil {
		return err
	}
	return updateBindingsAt(context.Background(), path, func(bindings *bindingsFile) (bool, error) {
		workspace, ok := bindings.Workspaces[canonicalRoot]
		if !ok {
			return false, nil
		}
		selection, ok := workspace.Environments[environment]
		if !ok {
			return false, nil
		}
		sectionKey := SectionKey(domain, backend)
		changed := false
		if projectName == "" {
			if _, ok := selection.Profiles[sectionKey]; ok {
				delete(selection.Profiles, sectionKey)
				changed = true
			}
		} else if project, ok := selection.Projects[projectName]; ok {
			if _, ok := project.Profiles[sectionKey]; ok {
				delete(project.Profiles, sectionKey)
				changed = true
			}
			if len(project.Profiles) == 0 {
				delete(selection.Projects, projectName)
			} else {
				selection.Projects[projectName] = project
			}
		}
		if !changed {
			return false, nil
		}
		if len(selection.Profiles) == 0 && len(selection.Projects) == 0 {
			delete(workspace.Environments, environment)
		} else {
			workspace.Environments[environment] = selection
		}
		if len(workspace.Environments) == 0 {
			delete(bindings.Workspaces, canonicalRoot)
		} else {
			bindings.Workspaces[canonicalRoot] = workspace
		}
		return true, nil
	})
}

type environmentProfileBindingReference struct {
	WorkspaceRoot string `json:"workspaceRoot"`
	Environment   string `json:"environment"`
	Project       string `json:"project,omitempty"`
}

// withEnvironmentProfileBindingReferences reads every environment-aware
// reference to one typed Profile and holds the store's shared process + file
// locks until inspect returns. Remove performs its config Save inside inspect:
// a concurrent binder cannot slip between the precondition and deletion.
func withEnvironmentProfileBindingReferences(
	domain Domain, backend, name string,
	inspect func([]environmentProfileBindingReference) error,
) (err error) {
	if err := validateBackend(domain, backend); err != nil {
		return err
	}
	if err := ValidateName(name); err != nil {
		return err
	}
	if inspect == nil {
		return errors.New("profile bindings: reference inspector is required")
	}

	bindingsMu.RLock()
	defer bindingsMu.RUnlock()

	path, err := BindingsPath()
	if err != nil {
		return err
	}
	release, err := acquireBindingsFileLock(context.Background(), path, false)
	if err != nil {
		return err
	}
	defer func() {
		if releaseErr := release(); releaseErr != nil {
			err = errors.Join(err, releaseErr)
		}
	}()
	bindings, err := loadBindingsAt(path)
	if err != nil {
		return err
	}
	sectionKey := SectionKey(domain, backend)
	references := make([]environmentProfileBindingReference, 0)
	for root, workspace := range bindings.Workspaces {
		for environmentName, selection := range workspace.Environments {
			if selection.Profiles[sectionKey] == name {
				references = append(references, environmentProfileBindingReference{
					WorkspaceRoot: root,
					Environment:   environmentName,
				})
			}
			for projectName, project := range selection.Projects {
				if project.Profiles[sectionKey] == name {
					references = append(references, environmentProfileBindingReference{
						WorkspaceRoot: root,
						Environment:   environmentName,
						Project:       projectName,
					})
				}
			}
		}
	}
	sort.Slice(references, func(i, j int) bool {
		left := references[i]
		right := references[j]
		if left.WorkspaceRoot != right.WorkspaceRoot {
			return left.WorkspaceRoot < right.WorkspaceRoot
		}
		if left.Environment != right.Environment {
			return left.Environment < right.Environment
		}
		return left.Project < right.Project
	})
	return inspect(references)
}

// updateBindingsAt holds an exclusive cross-process lock for the complete
// read-modify-write transaction. Each call constructs an independent flock
// value so separate one serve processes coordinate through the sibling lock
// file rather than relying on process memory.
func updateBindingsAt(
	ctx context.Context,
	path string,
	mutate func(*bindingsFile) (bool, error),
) (err error) {
	if mutate == nil {
		return errors.New("profile bindings: mutate function is required")
	}
	release, err := acquireBindingsFileLock(ctx, path, true)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, release())
	}()

	bindings, err := loadBindingsAt(path)
	if err != nil {
		return err
	}
	changed, err := mutate(bindings)
	if err != nil || !changed {
		return err
	}
	return saveBindingsAt(bindings, path)
}

// readBindingsAt prevents a reader from observing the destination between a
// competing process's RMW load and atomic publication. Shared locks allow
// independent readers to proceed concurrently.
func readBindingsAt(ctx context.Context, path string) (bindings *bindingsFile, err error) {
	release, err := acquireBindingsFileLock(ctx, path, false)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errors.Join(err, release())
	}()
	return loadBindingsAt(path)
}

func acquireBindingsFileLock(
	ctx context.Context,
	path string,
	exclusive bool,
) (func() error, error) {
	if strings.TrimSpace(path) == "" || strings.ContainsRune(path, 0) {
		return nil, errors.New("profile bindings: path is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	lockContext := ctx
	cancel := func() {}
	if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > bindingsLockTimeout {
		lockContext, cancel = context.WithTimeout(ctx, bindingsLockTimeout)
	}
	defer cancel()

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create profile bindings directory: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return nil, fmt.Errorf("secure profile bindings directory: %w", err)
	}

	fileLock := flock.New(path + ".lock")
	var locked bool
	var err error
	if exclusive {
		locked, err = fileLock.TryLockContext(lockContext, bindingsLockRetryDelay)
	} else {
		locked, err = fileLock.TryRLockContext(lockContext, bindingsLockRetryDelay)
	}
	if err != nil {
		return nil, fmt.Errorf("lock profile bindings: %w", err)
	}
	if !locked {
		lockErr := lockContext.Err()
		if lockErr == nil {
			lockErr = errors.New("lock was not acquired")
		}
		return nil, fmt.Errorf("lock profile bindings: %w", lockErr)
	}
	if err := os.Chmod(fileLock.Path(), 0o600); err != nil {
		_ = fileLock.Unlock()
		return nil, fmt.Errorf("secure profile bindings lock: %w", err)
	}
	return func() error {
		if err := fileLock.Unlock(); err != nil {
			return fmt.Errorf("unlock profile bindings: %w", err)
		}
		return nil
	}, nil
}

func loadBindingsAt(path string) (*bindingsFile, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &bindingsFile{Version: bindingsSchemaVersion}, nil
		}
		return nil, err
	}
	var probe struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil, invalidBindingsFile(path, err)
	}
	if probe.Version != bindingsSchemaVersion {
		return nil, cliErrors.New(cliErrors.PROFILE_VERSION_UNSUPPORTED,
			fmt.Sprintf("profile-bindings.json schema version 不支持：要求 v%d，当前 v%d", bindingsSchemaVersion, probe.Version)).
			WithContext(map[string]any{"path": path, "version": probe.Version})
	}
	var bindings bindingsFile
	if err := json.Unmarshal(raw, &bindings); err != nil {
		return nil, invalidBindingsFile(path, err)
	}
	if bindings.Workspaces == nil {
		bindings.Workspaces = make(map[string]bindingsWorkspace)
	}
	return &bindings, nil
}

func invalidBindingsFile(path string, err error) error {
	return cliErrors.New(cliErrors.PROFILE_FILE_INVALID,
		"~/.config/one/profile-bindings.json 解析失败："+err.Error()).
		WithContext(map[string]any{"path": path})
}

func saveBindingsAt(bindings *bindingsFile, path string) error {
	if bindings == nil {
		return errors.New("profile: nil bindings")
	}
	bindings.Version = bindingsSchemaVersion
	if len(bindings.Workspaces) == 0 {
		bindings.Workspaces = nil
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return atomicWriteSynced(bindings, path)
}

// atomicWriteSynced makes the binding update durable without exposing a
// partially-written JSON document: write a sibling temp file, force its mode,
// fsync it, rename it over the destination, then fsync the parent directory.
func atomicWriteSynced(value any, path string) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".profile-bindings-*.json")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	closed := false
	defer func() {
		if !closed {
			_ = tmp.Close()
		}
		_ = os.Remove(tmpPath)
	}()
	if _, err := tmp.Write(raw); err != nil {
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		closed = true
		return err
	}
	closed = true
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	directory, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func canonicalBindingRoot(root string) (string, error) {
	if root != strings.TrimSpace(root) || root == "" || strings.ContainsRune(root, 0) {
		return "", invalidBindingValue("workspace root", root)
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", invalidBindingValue("workspace root", root)
	}
	abs = filepath.Clean(abs)
	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		return "", invalidBindingValue("workspace root", root)
	}
	canonical, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", invalidBindingValue("workspace root", root)
	}
	return filepath.Clean(canonical), nil
}

func validateBindingEnvironment(environment string) (string, error) {
	if environment != strings.TrimSpace(environment) || len(environment) > 128 ||
		!bindingIdentifierRE.MatchString(environment) {
		return "", invalidBindingValue("environment", environment)
	}
	return environment, nil
}

func validateBindingProfileName(name string) (string, error) {
	if err := ValidateName(name); err != nil {
		return "", err
	}
	return name, nil
}

func validateBindingMetadata(field, value string, optional bool) (string, error) {
	if value != strings.TrimSpace(value) || len(value) > 256 {
		return "", invalidBindingValue(field, value)
	}
	if value == "" && optional {
		return "", nil
	}
	for _, char := range value {
		if unicode.IsControl(char) {
			return "", invalidBindingValue(field, value)
		}
	}
	return value, nil
}

func invalidBindingValue(field, value string) error {
	return cliErrors.New(cliErrors.PROFILE_BACKEND_INVALID,
		fmt.Sprintf("%s 不合法。", field)).
		WithContext(map[string]any{"field": field, "value": value})
}

func environmentBindingNamesAt(
	root, environment, projectName, sectionKey string,
) (projectNameResult, workspaceNameResult string, err error) {
	bindings, err := environmentBindings(root, environment)
	if err != nil || bindings == nil {
		return "", "", err
	}
	workspaceNameResult = strings.TrimSpace(bindings.Profiles[sectionKey])
	projectName = strings.TrimSpace(projectName)
	if projectName == "" {
		return "", workspaceNameResult, nil
	}
	project, ok := bindings.Projects[projectName]
	if !ok {
		return "", workspaceNameResult, nil
	}
	return strings.TrimSpace(project.Profiles[sectionKey]), workspaceNameResult, nil
}

func environmentBindings(root, environment string) (*bindingsEnvironment, error) {
	canonicalRoot, err := canonicalBindingRoot(root)
	if err != nil {
		return nil, err
	}
	environment, err = validateBindingEnvironment(environment)
	if err != nil {
		return nil, err
	}
	bindingsMu.RLock()
	defer bindingsMu.RUnlock()
	path, err := BindingsPath()
	if err != nil {
		return nil, err
	}
	bindings, err := readBindingsAt(context.Background(), path)
	if err != nil {
		return nil, err
	}
	workspace, ok := bindings.Workspaces[canonicalRoot]
	if !ok {
		return nil, nil
	}
	selection, ok := workspace.Environments[environment]
	if !ok {
		return nil, nil
	}
	return &selection, nil
}
