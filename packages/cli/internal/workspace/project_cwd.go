package workspace

// project_cwd.go: pure cwd → project lookup. Belongs to the workspace
// package because it composes ResolveRootDirs + DiscoverProjects with a
// path-prefix filter — none of which is secrets-, infra-, or
// product-specific. Originally lived under
// internal/secrets/infisical/paths.go for historical reasons; moved here
// so that other domains (dotenv, future secrets backends, infra commands
// needing cwd-relative project resolution) can call it without depending
// on infisical.

import (
	"path/filepath"
	"strings"
)

// ResolveProjectFromCWD figures out which project (if any) the user is
// "inside" when running a command without -p / --project. Used so an
// engineer can `cd services/user-api` and run env / run / container
// commands without restating the selector.
//
// Manifest-driven: walks manifest.projects[] and matches cwd against
// each entry's relativeDir. Filesystem discovery is not used here — the
// manifest is the canonical source of truth, and matching against it
// means a project works the moment it's added to the manifest, even
// before any code is in the directory.
//
// Returns nil if cwd is not under any declared project.
func ResolveProjectFromCWD(projectRoot, cwd string) (*Project, error) {
	m, err := ReadManifest(projectRoot)
	if err != nil {
		return nil, err
	}
	// Resolve symlinks on both ends so macOS /var ↔ /private/var and
	// any user-side symlinked workspace dirs don't cause false misses.
	cwdResolved := canonicalize(cwd)
	rootResolved := canonicalize(projectRoot)
	// Pick the deepest match so nested projects (rare but legal)
	// resolve to the inner one — `cd apps/web/sub` should pick `web`,
	// then if a `web/sub` project also exists, pick that.
	var best *ManifestProject
	bestDepth := -1
	for i := range m.Projects {
		p := &m.Projects[i]
		target := filepath.Clean(filepath.Join(rootResolved, filepath.FromSlash(p.RelativeDir)))
		if cwdResolved != target && !strings.HasPrefix(cwdResolved, target+string(filepath.Separator)) {
			continue
		}
		depth := strings.Count(p.RelativeDir, "/")
		if depth > bestDepth {
			bestDepth = depth
			best = p
		}
	}
	if best == nil {
		return nil, nil
	}
	return manifestEntryToProject(projectRoot, *best), nil
}

// canonicalize returns the absolute, symlink-resolved form of path.
// Falls back to filepath.Clean when EvalSymlinks fails (e.g. path
// doesn't exist yet) so callers always get a usable string.
func canonicalize(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return filepath.Clean(resolved)
	}
	return filepath.Clean(path)
}

// ResolveProjectFromSelector resolves a pnpm-style selector to a
// project by consulting the manifest. Lookup order:
//
//  1. Exact match against manifest.projects[i].name
//  2. Path-style match against manifest.projects[i].relativeDir
//     (tolerates leading `./` and trailing `/` for the pnpm habit
//     `pnpm -F ./apps/web`)
//
// Manifest is the source of truth here, not filesystem discovery —
// `one env set -p web` should work the moment the user adds the
// project to the manifest, before any code is in the directory.
//
// Returns nil + nil error when nothing matches; surface that as a
// clear "no such project" error in the CLI layer rather than here.
func ResolveProjectFromSelector(projectRoot, selector string) (*Project, error) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return nil, nil
	}
	m, err := ReadManifest(projectRoot)
	if err != nil {
		return nil, err
	}
	pathLike := strings.TrimSuffix(strings.TrimPrefix(selector, "./"), "/")
	pathLike = ToPosixPath(pathLike)
	for _, p := range m.Projects {
		if p.Name == selector {
			return manifestEntryToProject(projectRoot, p), nil
		}
	}
	for _, p := range m.Projects {
		if p.RelativeDir == pathLike {
			return manifestEntryToProject(projectRoot, p), nil
		}
	}
	return nil, nil
}

func manifestEntryToProject(projectRoot string, p ManifestProject) *Project {
	return &Project{
		Name:           p.Name,
		RelativeDir:    p.RelativeDir,
		TargetDir:      filepath.Join(projectRoot, filepath.FromSlash(p.RelativeDir)),
		Toolchain:      p.Toolchain,
		PackageManager: p.PackageManager,
		TemplateID:     p.TemplateID,
	}
}

// ProjectNames returns the declared project names from the manifest in
// stable order. Used by the CLI layer when telling the user "no such
// project 'foo' — known names: web / api".
func ProjectNames(m *Manifest) []string {
	if m == nil {
		return nil
	}
	out := make([]string, 0, len(m.Projects))
	for _, p := range m.Projects {
		out = append(out, p.Name)
	}
	return out
}
