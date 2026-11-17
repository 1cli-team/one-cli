package gateway

import (
	"context"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"time"

	gitmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/git"
)

var issueReferencePattern = regexp.MustCompile(`(?i)^(?:[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}|[a-z0-9][a-z0-9_-]{0,63}-[0-9]{1,16})$`)

func ValidIssueReference(issue string) bool {
	return issueReferencePattern.MatchString(issue)
}

type JobPage struct {
	Items     []JobSummary `json:"items"`
	Page      int          `json:"page"`
	PageCount int          `json:"pageCount"`
	Issue     string       `json:"issue,omitempty"`
}

type JobSummary struct {
	ID           string          `json:"id"`
	ProjectID    string          `json:"projectId"`
	Identifier   string          `json:"identifier"`
	Title        string          `json:"title"`
	ProjectName  string          `json:"projectName,omitempty"`
	Description  string          `json:"description,omitempty"`
	URL          string          `json:"url,omitempty"`
	Stage        string          `json:"stage"`
	CreatedAt    time.Time       `json:"createdAt"`
	LastError    string          `json:"lastError,omitempty"`
	Codex        CodexOptions    `json:"codex,omitzero"`
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
	return c.IssueJobs(ctx, page, "")
}

func (c *Client) IssueJobs(ctx context.Context, page int, issue string) (JobPage, error) {
	var result JobPage
	endpoint := c.base.JoinPath("v1", "jobs")
	query := url.Values{"page": {strconv.Itoa(page)}}
	if issue != "" {
		query.Set("issue", issue)
	}
	endpoint.RawQuery = query.Encode()
	err := c.send(ctx, http.MethodGet, endpoint, nil, &result)
	return result, err
}
