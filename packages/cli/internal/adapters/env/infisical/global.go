package infisical

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"

	sdk "github.com/infisical/go-sdk"
	"github.com/infisical/go-sdk/packages/models"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/fsutil"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	session "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/infisicalsession"
)

type RemoteEnvironment struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}
type RemoteProject struct {
	ID             string              `json:"id"`
	LegacyID       string              `json:"_id,omitempty"`
	Name           string              `json:"name"`
	Type           string              `json:"type"`
	OrganizationID string              `json:"orgId"`
	Environments   []RemoteEnvironment `json:"environments"`
}

func Projects(ctx context.Context) ([]RemoteProject, error) {
	s, e := session.Require()
	if e != nil {
		return nil, e
	}
	return projectsFor(ctx, s)
}
func projectsFor(ctx context.Context, s *session.Session) ([]RemoteProject, error) {
	var result struct {
		Projects []RemoteProject `json:"workspaces"`
	}
	if e := session.Request(ctx, s, http.MethodGet, "/api/v1/workspace", nil, &result); e != nil {
		return nil, e
	}
	projects := []RemoteProject{}
	for _, p := range result.Projects {
		if p.ID == "" {
			p.ID = p.LegacyID
		}
		p.LegacyID = ""
		if p.Type == "secret-manager" && (s.OrganizationID == "" || p.OrganizationID == s.OrganizationID) {
			projects = append(projects, p)
		}
	}
	sort.Slice(projects, func(i, j int) bool { return projects[i].Name < projects[j].Name })
	return projects, nil
}
func Project(ctx context.Context, id string) (*RemoteProject, error) {
	s, e := session.Require()
	if e != nil {
		return nil, e
	}
	return projectFor(ctx, s, id)
}

// ProjectWithSession keeps metadata reads scoped to the binding's session.
func ProjectWithSession(ctx context.Context, s *session.Session, id string) (*RemoteProject, error) {
	return projectFor(ctx, s, id)
}

func projectFor(ctx context.Context, s *session.Session, id string) (*RemoteProject, error) {
	if strings.TrimSpace(id) == "" {
		return nil, i18n.Errorf("infisical.project_required")
	}
	var result struct {
		Project RemoteProject `json:"workspace"`
	}
	if e := session.Request(ctx, s, http.MethodGet, "/api/v1/workspace/"+url.PathEscape(id), nil, &result); e != nil {
		return nil, e
	}
	p := result.Project
	if p.ID == "" {
		p.ID = p.LegacyID
	}
	p.LegacyID = ""
	if p.ID != id {
		return nil, i18n.Errorf("infisical.project_response_invalid")
	}
	if s.OrganizationID != "" && p.OrganizationID != s.OrganizationID {
		return nil, i18n.Errorf("infisical.organization_mismatch")
	}
	if p.Type != "secret-manager" {
		return nil, i18n.Errorf("infisical.secret_manager_required")
	}
	return &p, nil
}

type GlobalLocation struct {
	SiteURL            string `json:"siteUrl"`
	UserID             string `json:"userId"`
	OrganizationID     string `json:"organizationId"`
	ProjectID          string `json:"projectId"`
	ProjectName        string `json:"projectName"`
	DefaultEnvironment string `json:"defaultEnvironment"`
}

func LoadGlobalLocation() (*GlobalLocation, error) {
	p, e := session.ConfigPath("global-env.json")
	if e != nil {
		return nil, e
	}
	data, e := os.ReadFile(p)
	if os.IsNotExist(e) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	var location GlobalLocation
	if json.Unmarshal(data, &location) != nil {
		return nil, i18n.Errorf("global.location_invalid")
	}
	return &location, nil
}
func BindGlobal(ctx context.Context, projectID, env string) (*GlobalLocation, error) {
	return withLocationLock(ctx, func() (*GlobalLocation, error) {
		s, e := session.Require()
		if e != nil {
			return nil, e
		}
		return bindGlobalFor(ctx, s, projectID, env)
	})
}
func bindGlobalFor(ctx context.Context, s *session.Session, projectID, env string) (*GlobalLocation, error) {
	p, e := projectFor(ctx, s, projectID)
	if e != nil {
		return nil, e
	}
	if e = validateRemoteEnvironment(p, env); e != nil {
		return nil, e
	}
	location := &GlobalLocation{SiteURL: s.SiteURL, UserID: s.UserID, OrganizationID: p.OrganizationID, ProjectID: p.ID, ProjectName: p.Name, DefaultEnvironment: env}
	file, e := session.ConfigPath("global-env.json")
	if e != nil {
		return nil, e
	}
	data, _ := json.MarshalIndent(location, "", "  ")
	if e = session.WithUnchanged(s, func() error {
		return fsutil.WriteAtomic(file, append(data, '\n'), 0600)
	}); e != nil {
		return nil, e
	}
	return location, nil
}
func validateRemoteEnvironment(p *RemoteProject, env string) error {
	for _, v := range p.Environments {
		if v.Slug == env {
			return nil
		}
	}
	return i18n.Errorf("global.environment_missing", p.Name, env)
}
func ValidateGlobalPath(raw string) (string, error) {
	if raw == "" {
		raw = "/"
	}
	if !strings.HasPrefix(raw, "/") || strings.ContainsAny(raw, "\\\x00\r\n") {
		return "", i18n.Errorf("global.path_absolute")
	}
	for _, part := range strings.Split(raw, "/") {
		if part == ".." || part == "." {
			return "", i18n.Errorf("global.path_segments")
		}
	}
	return path.Clean(raw), nil
}
func globalClient(ctx context.Context, env string) (*Client, *GlobalLocation, string, error) {
	s, e := session.Require()
	if e != nil {
		return nil, nil, "", e
	}
	location, e := LoadGlobalLocation()
	if e != nil {
		return nil, nil, "", e
	}
	if location == nil {
		return nil, nil, "", i18n.Errorf("global.location_required")
	}
	if location.SiteURL != s.SiteURL || location.UserID != s.UserID || (s.OrganizationID != "" && location.OrganizationID != s.OrganizationID) {
		return nil, nil, "", i18n.Errorf("global.location_mismatch")
	}
	p, e := projectFor(ctx, s, location.ProjectID)
	if e != nil {
		return nil, nil, "", e
	}
	if p.OrganizationID != location.OrganizationID {
		return nil, nil, "", i18n.Errorf("global.organization_changed")
	}
	if env == "" {
		env = location.DefaultEnvironment
	}
	if e = validateRemoteEnvironment(p, env); e != nil {
		return nil, nil, "", e
	}
	c, e := NewClient(ctx, &WorkspaceConfig{SiteURL: s.SiteURL, ProjectID: p.ID}, &Credentials{AccessToken: s.Token})
	return c, location, env, e
}

type GlobalEntry struct {
	Key         string `json:"key"`
	Description string `json:"description,omitempty"`
}
type GlobalListing struct {
	Location    *GlobalLocation `json:"location"`
	Environment string          `json:"environment"`
	Path        string          `json:"path"`
	Folders     []string        `json:"folders"`
	Variables   []GlobalEntry   `json:"variables"`
}

func ListGlobal(ctx context.Context, env, folder string) (*GlobalListing, error) {
	folder, e := ValidateGlobalPath(folder)
	if e != nil {
		return nil, e
	}
	c, l, env, e := globalClient(ctx, env)
	if e != nil {
		return nil, e
	}
	dirs, e := c.sdk.Folders().List(sdk.ListFoldersOptions{ProjectID: l.ProjectID, Environment: env, Path: folder})
	if e != nil {
		return nil, mapAPIError(e)
	}
	values, e := c.sdk.Secrets().List(sdk.ListSecretsOptions{ProjectID: l.ProjectID, Environment: env, SecretPath: folder, Recursive: false, ExpandSecretReferences: false})
	if e != nil {
		return nil, mapAPIError(e)
	}
	result := &GlobalListing{Location: l, Environment: env, Path: folder, Folders: []string{}, Variables: []GlobalEntry{}}
	for _, d := range dirs {
		result.Folders = append(result.Folders, path.Join(folder, d.Name))
	}
	for _, v := range values {
		result.Variables = append(result.Variables, GlobalEntry{Key: v.SecretKey, Description: v.SecretComment})
	}
	sort.Strings(result.Folders)
	sort.Slice(result.Variables, func(i, j int) bool { return result.Variables[i].Key < result.Variables[j].Key })
	return result, nil
}

var envKey = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func GlobalSecret(ctx context.Context, action, env, folder, key, value string) (any, error) {
	if !envKey.MatchString(key) {
		return nil, i18n.Errorf("global.key_format")
	}
	folder, e := ValidateGlobalPath(folder)
	if e != nil {
		return nil, e
	}
	c, _, env, e := globalClient(ctx, env)
	if e != nil {
		return nil, e
	}
	switch action {
	case "get":
		v, e := c.retrieveGlobalSecret(env, folder, key)
		if e != nil {
			return nil, e
		}
		return map[string]string{"key": key, "value": v.SecretValue, "environment": env, "path": folder}, nil
	case "create":
		_, e = c.CreateSecret(env, folder, key, value)
	case "update":
		_, e = c.UpdateSecret(env, folder, key, value)
	case "unset":
		_, e = c.DeleteSecret(env, folder, key)
	default:
		return nil, i18n.Errorf("global.operation_unsupported")
	}
	if e != nil {
		return nil, e
	}
	return map[string]string{"key": key, "environment": env, "path": folder, "action": action}, nil
}
func CreateGlobalFolder(ctx context.Context, env, folder, name string) error {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\\x00\r\n") {
		return i18n.Errorf("global.folder_invalid")
	}
	folder, e := ValidateGlobalPath(folder)
	if e != nil {
		return e
	}
	c, l, env, e := globalClient(ctx, env)
	if e != nil {
		return e
	}
	_, e = c.sdk.Folders().Create(sdk.CreateFolderOptions{ProjectID: l.ProjectID, Environment: env, Path: folder, Name: name})
	return mapAPIError(e)
}
func GlobalValues(ctx context.Context, env, folder string, keys []string) (map[string]string, error) {
	if env == "" || folder == "" {
		return nil, i18n.Errorf("global.scope_required")
	}
	folder, e := ValidateGlobalPath(folder)
	if e != nil {
		return nil, e
	}
	c, l, env, e := globalClient(ctx, env)
	if e != nil {
		return nil, e
	}
	vars := map[string]string{}
	// Selected keys are fetched individually; other values never enter this process.
	if len(keys) > 0 {
		for _, k := range keys {
			if !envKey.MatchString(k) {
				return nil, i18n.Errorf("global.key_invalid")
			}
			v, e := c.retrieveGlobalSecret(env, folder, k)
			if e != nil {
				return nil, e
			}
			vars[k] = v.SecretValue
		}
		return vars, nil
	}
	values, e := c.sdk.Secrets().List(sdk.ListSecretsOptions{ProjectID: l.ProjectID, Environment: env, SecretPath: folder, Recursive: false, ExpandSecretReferences: false})
	if e != nil {
		return nil, mapAPIError(e)
	}
	for _, v := range values {
		vars[v.SecretKey] = v.SecretValue
	}
	return vars, nil
}

func (c *Client) retrieveGlobalSecret(env, folder, key string) (*models.Secret, error) {
	v, err := c.sdk.Secrets().Retrieve(sdk.RetrieveSecretOptions{ProjectID: c.cfg.ProjectID, Environment: env, SecretPath: folder, SecretKey: key, ExpandSecretReferences: false})
	if err != nil {
		return nil, mapAPIError(err)
	}
	return &v, nil
}
func GlobalSummary(ctx context.Context) (*GlobalLocation, []RemoteEnvironment, error) {
	_, location, _, err := globalClient(ctx, "")
	if err != nil {
		return nil, nil, err
	}
	p, err := Project(ctx, location.ProjectID)
	if err != nil {
		return nil, nil, err
	}
	return location, p.Environments, nil
}
