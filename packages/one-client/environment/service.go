// Package environment adapts the management API to the CLI's environment workflow.
package environment

import (
	"context"
	oneclient "github.com/torchstellar-team/one-cli/packages/one-client"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/application/execution"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/workspace"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"net/url"
	pathpkg "path"
	"path/filepath"
	"strings"
)

type Service struct{ Client *oneclient.Client }
type GetInput struct {
	Scope                     execution.Scope
	Environment, Project, Key string
}
type ListInput struct {
	Scope                execution.Scope
	Environment, Project string
}
type DeleteInput struct {
	Scope                     execution.Scope
	Environment, Project, Key string
}
type DeleteResult struct {
	Schema string `json:"schema"`
	Env    string `json:"env"`
	Path   string `json:"path"`
	Key    string `json:"key"`
	Action string `json:"action"`
}
type PlanSetInput struct {
	Scope                execution.Scope
	Environment, Project string
}
type SetPlan struct {
	Environment              string   `json:"environment"`
	NeedsEnvironmentCreation bool     `json:"needsEnvironmentCreation"`
	ProjectChoices           []string `json:"projectChoices"`
	scope                    execution.Scope
	project                  string
}

func (p SetPlan) WithProject(project string) SetPlan {
	p.project = project
	p.ProjectChoices = nil
	return p
}

type SetInput struct {
	Plan                          SetPlan
	Key, Value                    string
	Overwrite, RepositoryReadOnly bool
}
type BindWorkspaceInput struct {
	Scope     execution.Scope
	ProjectID string
	Create    bool
	Revision  string
}

func rootAndProject(scope execution.Scope, project string) (string, string, error) {
	root := scope.WorkspaceRoot()
	var err error
	if root == "" {
		root, err = workspace.WalkUpToManifest(scope.WorkingDirectory())
		if err != nil {
			return "", "", err
		}
	}
	if project == "" {
		project = scope.Project()
	}
	if project == "" {
		m, e := workspace.ReadManifest(root)
		if e != nil {
			return "", "", e
		}
		rel, e := filepath.Rel(root, scope.WorkingDirectory())
		if e == nil {
			rel = filepath.ToSlash(rel)
			best := 0
			for _, p := range m.Projects {
				if (rel == p.RelativeDir || strings.HasPrefix(rel, p.RelativeDir+"/")) && len(p.RelativeDir) > best {
					project = p.Name
					best = len(p.RelativeDir)
				}
			}
		}
	}
	if project != "" {
		project, err = oneclient.ProjectSelector(root, project)
		if err != nil {
			return "", "", err
		}
	}
	return root, project, nil
}
func (s *Service) call(ctx context.Context, scope execution.Scope, project, operation string, body map[string]any, out any) error {
	root, project, err := rootAndProject(scope, project)
	if err != nil {
		return err
	}
	body["project"] = project
	return s.Client.WorkspaceRequest(ctx, root, project, "POST", "/operations/"+operation, body, out)
}
func (s *Service) Summary(scope execution.Scope) (*Summary, error) {
	var r Summary
	err := s.call(scope.Context(), scope, "", "summary", map[string]any{}, &r)
	return &r, err
}
func (s *Service) Get(ctx context.Context, in GetInput) (*GetResult, error) {
	var r GetResult
	err := s.call(ctx, in.Scope, in.Project, "get", map[string]any{"environment": in.Environment, "key": in.Key}, &r)
	return &r, err
}
func (s *Service) List(ctx context.Context, in ListInput) (*ListResult, error) {
	var r ListResult
	err := s.call(ctx, in.Scope, in.Project, "list", map[string]any{"environment": in.Environment}, &r)
	return &r, err
}
func (s *Service) Delete(ctx context.Context, in DeleteInput) (*DeleteResult, error) {
	var r DeleteResult
	err := s.call(ctx, in.Scope, in.Project, "delete", map[string]any{"environment": in.Environment, "key": in.Key}, &r)
	return &r, err
}
func (s *Service) PlanSet(in PlanSetInput) (SetPlan, error) {
	var p SetPlan
	err := s.call(in.Scope.Context(), in.Scope, in.Project, "plan-set", map[string]any{"environment": in.Environment}, &p)
	_, project, e := rootAndProject(in.Scope, in.Project)
	if err == nil {
		err = e
	}
	p.scope = in.Scope
	p.project = project
	return p, err
}
func (s *Service) Set(ctx context.Context, in SetInput) (*SetResult, error) {
	var r SetResult
	err := s.call(ctx, in.Plan.scope, in.Plan.project, "set", map[string]any{"environment": in.Plan.Environment, "key": in.Key, "value": in.Value, "overwrite": in.Overwrite}, &r)
	return &r, err
}
func (s *Service) BindWorkspace(ctx context.Context, in BindWorkspaceInput) (*WorkspaceBindingResult, error) {
	var r WorkspaceBindingResult
	err := s.call(ctx, in.Scope, "", "bind", map[string]any{"projectId": in.ProjectID, "create": in.Create, "revision": in.Revision}, &r)
	return &r, err
}

type RemoteEnvironment struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}
type RemoteProject struct {
	ID             string              `json:"id"`
	Name           string              `json:"name"`
	Type           string              `json:"type"`
	OrganizationID string              `json:"orgId"`
	Environments   []RemoteEnvironment `json:"environments"`
}
type GlobalLocation struct {
	SiteURL            string `json:"siteUrl"`
	UserID             string `json:"userId"`
	OrganizationID     string `json:"organizationId"`
	ProjectID          string `json:"projectId"`
	ProjectName        string `json:"projectName"`
	DefaultEnvironment string `json:"defaultEnvironment"`
}
type GlobalVariable struct {
	Key         string `json:"key"`
	Description string `json:"description,omitempty"`
}
type GlobalListing struct {
	Location    GlobalLocation   `json:"location"`
	Environment string           `json:"environment"`
	Path        string           `json:"path"`
	Folders     []string         `json:"folders"`
	Variables   []GlobalVariable `json:"variables"`
}

func Projects(ctx context.Context) ([]RemoteProject, error) {
	var r []RemoteProject
	err := oneclient.New().DefaultRequest(ctx, "GET", "/infisical/projects", nil, &r)
	return r, err
}
func Project(ctx context.Context, id string) (*RemoteProject, error) {
	var r RemoteProject
	err := oneclient.New().DefaultRequest(ctx, "GET", "/infisical/projects/"+url.PathEscape(id), nil, &r)
	return &r, err
}
func CreateRemoteProject(ctx context.Context, name string) (*RemoteProject, error) {
	var r RemoteProject
	err := oneclient.New().DefaultRequest(ctx, "POST", "/infisical/projects", map[string]string{"name": name}, &r)
	return &r, err
}
func GlobalSummary(ctx context.Context) (*GlobalLocation, []RemoteEnvironment, error) {
	var r struct {
		Location     GlobalLocation      `json:"location"`
		Environments []RemoteEnvironment `json:"environments"`
	}
	err := oneclient.New().DefaultRequest(ctx, "GET", "/global-env/location", nil, &r)
	return &r.Location, r.Environments, err
}
func BindGlobal(ctx context.Context, id, env string) (*GlobalLocation, error) {
	var r GlobalLocation
	err := oneclient.New().DefaultRequest(ctx, "PUT", "/global-env/location", map[string]string{"projectId": id, "environment": env}, &r)
	return &r, err
}
func BindDefaultGlobal(ctx context.Context, env string) (*GlobalLocation, error) {
	var r struct {
		SharedCredentials struct {
			Location *GlobalLocation `json:"location"`
		} `json:"sharedCredentials"`
	}
	err := oneclient.New().DefaultRequest(ctx, "POST", "/global-env/location/default", map[string]string{"environment": env}, &r)
	return r.SharedCredentials.Location, err
}
func GlobalValues(ctx context.Context, env, path string, keys []string) (map[string]string, error) {
	var r map[string]string
	err := oneclient.New().DefaultRequest(ctx, "POST", "/global-env/values", map[string]any{"environment": env, "path": path, "keys": keys}, &r)
	return r, err
}
func LoadGlobalLocation() (*GlobalLocation, error) {
	location, _, err := GlobalSummary(context.Background())
	return location, err
}
func ListGlobal(ctx context.Context, env, path string) (*GlobalListing, error) {
	var r GlobalListing
	err := oneclient.New().DefaultRequest(ctx, "GET", "/global-env/secrets?"+url.Values{"env": {env}, "path": {path}}.Encode(), nil, &r)
	return &r, err
}
func GlobalSecret(ctx context.Context, action, env, path, key, value string) (any, error) {
	method := map[string]string{"get": "GET", "create": "POST", "update": "PUT", "unset": "DELETE"}[action]
	var r any
	var body any
	if method == "POST" || method == "PUT" {
		body = map[string]string{"value": value}
	}
	err := oneclient.New().DefaultRequest(ctx, method, "/global-env/secrets/"+url.PathEscape(key)+"?"+url.Values{"env": {env}, "path": {path}}.Encode(), body, &r)
	return r, err
}
func ValidateGlobalPath(value string) (string, error) {
	if value == "" {
		return "/", nil
	}
	if !strings.HasPrefix(value, "/") || strings.ContainsAny(value, "\\\x00\r\n") {
		return "", i18n.Errorf("global.path_absolute")
	}
	for _, part := range strings.Split(value, "/") {
		if part == ".." || part == "." {
			return "", i18n.Errorf("global.path_segments")
		}
	}
	return pathpkg.Clean(value), nil
}

func (s *Service) Projects(scope execution.Scope) ([]RemoteProject, error) {
	root, project, err := rootAndProject(scope, "")
	if err != nil {
		return nil, err
	}
	connection, _, err := s.Client.Register(scope.Context(), root, project)
	if err != nil {
		return nil, err
	}
	var projects []RemoteProject
	err = s.Client.Request(scope.Context(), connection, "GET", "/infisical/projects", nil, &projects)
	return projects, err
}
