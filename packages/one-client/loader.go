package oneclient

import (
	"context"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/workspace"
	"net/url"
	"os"
	"path/filepath"
)

// Loader fetches values afresh for each invocation and keeps each project's
// server selection separate. It implements Core's process-injection port.
type Loader struct{ Client *Client }

func (l Loader) ID() string { return "infisical" }
func (l Loader) Enabled(ctx context.Context, root, dir string) (bool, error) {
	project, err := projectName(root, dir)
	if err != nil {
		return false, err
	}
	// A fresh, unconfigured CLI can execute local tasks before a management
	// service is installed. Once any managed configuration exists, errors must
	// be surfaced rather than silently running without its environment values.
	m, err := workspace.ReadManifest(root)
	if err != nil {
		return false, err
	}
	p, err := l.Client.ReadConnections()
	if err != nil {
		return false, err
	}
	_, bound := p.Bindings[root]
	if m.Workspace == nil && m.Env == nil && !bound && os.Getenv("ONE_SERVER_URL") == "" {
		return false, nil
	}
	_, dbErr := os.Stat(filepath.Join(l.Client.Root, "data", "one.db"))
	if m.Env == nil && !bound && p.Default == "local" && os.Getenv("ONE_SERVER_URL") == "" && os.IsNotExist(dbErr) {
		return false, nil
	}
	_, s, err := l.Client.Register(ctx, root, project)
	if err != nil {
		return false, err
	}
	return s.Manifest.Env != nil && s.Manifest.Env.ProjectID != "", nil
}
func (l Loader) Load(ctx context.Context, root, dir, env string) (map[string]string, error) {
	values, err := l.LoadProjects(ctx, root, []string{dir}, env)
	if err != nil {
		return nil, err
	}
	return values[dir], nil
}
func (l Loader) LoadProjects(ctx context.Context, root string, dirs []string, env string) (map[string]map[string]string, error) {
	out := map[string]map[string]string{}
	type group struct {
		connection Connection
		snapshot   Snapshot
		dirs       []string
	}
	groups := map[string]*group{}
	for _, dir := range dirs {
		project, err := projectName(root, dir)
		if err != nil {
			return nil, err
		}
		conn, s, err := l.Client.Register(ctx, root, project)
		if err != nil {
			return nil, err
		}
		key := conn.URL + "\x00" + s.EntryID
		g := groups[key]
		if g == nil {
			g = &group{connection: conn, snapshot: s}
			groups[key] = g
		}
		g.dirs = append(g.dirs, dir)
	}
	for _, g := range groups {
		var values map[string]map[string]string
		err := l.Client.Request(ctx, g.connection, "POST", "/workspaces/"+url.PathEscape(g.snapshot.EntryID)+"/operations/snapshot", map[string]any{"environment": env, "dirs": g.dirs}, &values)
		if err != nil {
			return nil, err
		}
		for dir, value := range values {
			out[dir] = value
		}
	}
	return out, nil
}
func projectName(root, dir string) (string, error) {
	m, err := workspace.ReadManifest(root)
	if err != nil {
		return "", err
	}
	for _, p := range m.Projects {
		if p.RelativeDir == dir {
			return p.Name, nil
		}
	}
	return "", nil
}

// ProjectSelector normalizes client paths before connection selection and
// transport. A server receives logical project names rather than local paths.
func ProjectSelector(root, selector string) (string, error) {
	if selector == "" {
		return "", nil
	}
	m, err := workspace.ReadManifest(root)
	if err != nil {
		return "", err
	}
	for _, project := range m.Projects {
		if selector == project.Name || filepath.ToSlash(filepath.Clean(selector)) == project.RelativeDir || filepath.Clean(selector) == filepath.Join(root, filepath.FromSlash(project.RelativeDir)) {
			return project.Name, nil
		}
	}
	return selector, nil
}
