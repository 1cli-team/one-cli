package gateway

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"

	gitmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/git"
)

type JobPage struct {
	Items     []JobSummary `json:"items"`
	Page      int          `json:"page"`
	PageCount int          `json:"pageCount"`
}

type JobSummary struct {
	ID           string          `json:"id"`
	Identifier   string          `json:"identifier"`
	Title        string          `json:"title"`
	ProjectName  string          `json:"projectName,omitempty"`
	Description  string          `json:"description,omitempty"`
	URL          string          `json:"url,omitempty"`
	Stage        string          `json:"stage"`
	CreatedAt    time.Time       `json:"createdAt"`
	LastError    string          `json:"lastError,omitempty"`
	Repositories []JobRepository `json:"repositories"`
}

type JobRepository struct {
	Repository gitmodule.RepositoryRef `json:"repository"`
	Base       string                  `json:"base,omitempty"`
	Branch     string                  `json:"branch,omitempty"`
	Unchanged  bool                    `json:"unchanged,omitempty"`
	PR         *JobPullRequest         `json:"pr,omitempty"`
}

type JobPullRequest struct {
	Number int    `json:"number"`
	URL    string `json:"url"`
	State  string `json:"state"`
	Merged bool   `json:"merged"`
}

func (c *Client) MonitorJobs(ctx context.Context, page int) (JobPage, error) {
	var result JobPage
	endpoint := c.base.JoinPath("v1", "jobs")
	endpoint.RawQuery = url.Values{"page": {strconv.Itoa(page)}}.Encode()
	err := c.send(ctx, http.MethodGet, endpoint, nil, &result)
	return result, err
}
