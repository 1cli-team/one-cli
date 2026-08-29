package execution

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
)

// Workspace is the command-scoped workspace snapshot for one execution.
// It resolves the filesystem boundary and manifest once; transports pass the
// snapshot downward instead of rediscovering the same state in each helper.
type Workspace struct {
	scope    Scope
	root     string
	manifest *workspace.Manifest
}

// ResolveWorkspace resolves the execution Scope carried by ctx. Commands that
// run outside the root harness still work by deriving a Scope from the process
// working directory.
func ResolveWorkspace(ctx context.Context) (Workspace, error) {
	scope, ok := FromContext(ctx)
	if !ok {
		workingDirectory, err := os.Getwd()
		if err != nil {
			return Workspace{}, err
		}
		scope = NewScope(ctx, workingDirectory)
	}
	return ResolveWorkspaceScope(scope)
}

// ResolveWorkspaceScope resolves a workspace from an explicit parent Scope.
func ResolveWorkspaceScope(scope Scope) (Workspace, error) {
	start := strings.TrimSpace(scope.WorkspaceRoot())
	if start == "" {
		start = strings.TrimSpace(scope.WorkingDirectory())
	}
	root, err := workspace.WalkUpToManifest(start)
	if err != nil {
		return Workspace{}, err
	}
	manifest, err := workspace.ReadManifest(root)
	if err != nil {
		return Workspace{}, err
	}
	return Workspace{
		scope: scope.Derive(ScopePatch{WorkspaceRoot: root}),
		root:  root, manifest: manifest,
	}, nil
}

func (w Workspace) Scope() Scope                  { return w.scope }
func (w Workspace) Root() string                  { return w.root }
func (w Workspace) Manifest() *workspace.Manifest { return w.manifest }

// Reload refreshes the snapshot after a workflow intentionally writes the
// manifest. Ordinary reads should keep using the existing snapshot.
func (w Workspace) Reload() (Workspace, error) {
	manifest, err := workspace.ReadManifest(w.root)
	if err != nil {
		return Workspace{}, err
	}
	w.manifest = manifest
	return w, nil
}

// Projects returns every manifest project with its absolute target path.
func (w Workspace) Projects() []workspace.Project {
	if w.manifest == nil {
		return nil
	}
	projects := make([]workspace.Project, 0, len(w.manifest.Projects))
	for _, project := range w.manifest.Projects {
		projects = append(projects, w.project(project))
	}
	return projects
}

func (w Workspace) ProjectNames() []string { return workspace.ProjectNames(w.manifest) }

// Project resolves a manifest name or relative path without reading the
// manifest again. Empty input falls back to the project already carried by the
// Scope, if any.
func (w Workspace) Project(selector string) (*workspace.Project, bool) {
	project, ok := w.manifestProject(selector)
	if !ok {
		return nil, false
	}
	resolved := w.project(*project)
	return &resolved, true
}

// manifestProject resolves the source manifest entry without another file
// read. The returned pointer belongs to this snapshot.
func (w Workspace) manifestProject(selector string) (*workspace.ManifestProject, bool) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		selector = strings.TrimSpace(w.scope.Project())
	}
	if selector == "" || w.manifest == nil {
		return nil, false
	}
	for index := range w.manifest.Projects {
		if w.manifest.Projects[index].Name == selector {
			return &w.manifest.Projects[index], true
		}
	}
	pathSelector := strings.TrimSuffix(strings.TrimPrefix(selector, "./"), "/")
	pathSelector = workspace.ToPosixPath(pathSelector)
	for index := range w.manifest.Projects {
		if w.manifest.Projects[index].RelativeDir == pathSelector {
			return &w.manifest.Projects[index], true
		}
	}
	return nil, false
}

// ProjectFromWorkingDirectory finds the deepest project containing the
// command's working directory, using the already-loaded manifest.
func (w Workspace) ProjectFromWorkingDirectory() (*workspace.Project, bool) {
	if w.manifest == nil {
		return nil, false
	}
	workingDirectory := canonicalPath(w.scope.WorkingDirectory())
	root := canonicalPath(w.root)
	bestIndex := -1
	bestDepth := -1
	for index, project := range w.manifest.Projects {
		target := filepath.Clean(filepath.Join(root, filepath.FromSlash(project.RelativeDir)))
		if workingDirectory != target && !strings.HasPrefix(workingDirectory, target+string(filepath.Separator)) {
			continue
		}
		depth := strings.Count(project.RelativeDir, "/")
		if depth > bestDepth {
			bestIndex = index
			bestDepth = depth
		}
	}
	if bestIndex < 0 {
		return nil, false
	}
	resolved := w.project(w.manifest.Projects[bestIndex])
	return &resolved, true
}

func (w Workspace) project(project workspace.ManifestProject) workspace.Project {
	return workspace.Project{
		Name: project.Name, RelativeDir: project.RelativeDir,
		TargetDir: filepath.Join(w.root, filepath.FromSlash(project.RelativeDir)),
		Toolchain: project.Toolchain, PackageManager: project.PackageManager,
		TemplateID: project.TemplateID,
	}
}

func canonicalPath(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return filepath.Clean(resolved)
	}
	return filepath.Clean(path)
}
