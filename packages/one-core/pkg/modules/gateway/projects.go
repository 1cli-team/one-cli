package gateway

import (
	"context"
	"net/http"
	"net/url"

	gitmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/git"
)

type RepositoryOption struct {
	ID         string                  `json:"id"`
	Name       string                  `json:"name"`
	Path       string                  `json:"path"`
	GitHub     string                  `json:"github"`
	Repository gitmodule.RepositoryRef `json:"repository,omitzero"`
}

func (r RepositoryOption) Ref() gitmodule.RepositoryRef {
	if r.Repository != (gitmodule.RepositoryRef{}) {
		return r.Repository
	}
	return gitmodule.RepositoryRef{Host: gitmodule.GitHubHost, Slug: r.GitHub}
}

type RepositoryChoices struct {
	Base     string   `json:"base"`
	Branches []string `json:"branches"`
	Tasks    []string `json:"tasks"`
}

type TeamFlow struct {
	Running string `json:"running"`
	Review  string `json:"review"`
	Done    string `json:"done"`
}

type ProjectMapping struct {
	RepositoryID  string              `json:"repositoryId"`
	RepositoryIDs []string            `json:"repositoryIds,omitempty"`
	Base          string              `json:"base"`
	CheckTask     string              `json:"checkTask"`
	ExistingCheck bool                `json:"existingCheck"`
	Teams         map[string]TeamFlow `json:"teams"`
}

type ProjectInput struct {
	OrganizationID string   `json:"organizationId"`
	ConnectionID   string   `json:"connectionId"`
	RepositoryID   string   `json:"repositoryId"`
	RepositoryIDs  []string `json:"repositoryIds,omitempty"`
	Base           string   `json:"base"`
	CheckTask      string   `json:"checkTask"`
	TeamID         string   `json:"teamId"`
	Flow           TeamFlow `json:"flow"`
}

func (c *Client) Repositories(ctx context.Context) ([]RepositoryOption, error) {
	var result []RepositoryOption
	err := c.do(ctx, http.MethodGet, &result, "repositories")
	return result, err
}

func (c *Client) RepositoryChoices(ctx context.Context, id string) (RepositoryChoices, error) {
	var result RepositoryChoices
	err := c.do(ctx, http.MethodGet, &result, "repositories", url.PathEscape(id))
	return result, err
}

func (c *Client) ProjectMapping(ctx context.Context, project string) (*ProjectMapping, error) {
	var result *ProjectMapping
	err := c.do(ctx, http.MethodGet, &result, "projects", url.PathEscape(project), "configuration")
	return result, err
}

func (c *Client) SaveProjectMapping(ctx context.Context, project string, input ProjectInput) (ProjectMapping, error) {
	var result ProjectMapping
	err := c.request(ctx, http.MethodPut, input, &result, "projects", url.PathEscape(project), "configuration")
	return result, err
}
