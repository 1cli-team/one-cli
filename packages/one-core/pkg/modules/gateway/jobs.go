package gateway

import (
	"context"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"time"

	gitmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/git"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/knowledge"
)

var identifierUUID = regexp.MustCompile(`(?i)^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$`)

func ValidIssueID(id string) bool { return identifierUUID.MatchString(id) }

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
	OpenViking         *knowledge.SessionLink `json:"openViking,omitempty"`
	RevisionCommentID  string                 `json:"revisionCommentId,omitempty"`
	RevisionIssueID    string                 `json:"revisionIssueId,omitempty"`
	CommitSHA          string                 `json:"commitSha,omitempty"`
	Subtasks           []SubtaskResult        `json:"subtasks,omitempty"`
	CanReadCodexResult bool                   `json:"canReadCodexResult,omitempty"`
	ID                 string                 `json:"id"`
	ProjectID          string                 `json:"projectId"`
	IssueID            string                 `json:"issueId"`
	Identifier         string                 `json:"identifier"`
	Title              string                 `json:"title"`
	ProjectName        string                 `json:"projectName,omitempty"`
	Description        string                 `json:"description,omitempty"`
	URL                string                 `json:"url,omitempty"`
	Stage              string                 `json:"stage"`
	CreatedAt          time.Time              `json:"createdAt"`
	UpdatedAt          time.Time              `json:"updatedAt,omitzero"`
	LastError          string                 `json:"lastError,omitempty"`
	Codex              CodexOptions           `json:"codex,omitzero"`
	Repositories       []JobRepository        `json:"repositories"`
	Question           *ExecutionQuestion     `json:"question,omitempty"`
	Progress           *ExecutionProgress     `json:"progress,omitempty"`
}

type SubtaskResult struct {
	Identifier string `json:"identifier"`
	Title      string `json:"title"`
	Stage      string `json:"stage"`
	CommitSHA  string `json:"commitSha,omitempty"`
	Response   string `json:"response,omitempty"`
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

type CodexJobResult struct {
	Response string `json:"response,omitempty"`
}

func (c *Client) JobCodexResult(ctx context.Context, id string) (CodexJobResult, error) {
	var result CodexJobResult
	if !identifierUUID.MatchString(id) {
		return result, &ResponseError{Code: http.StatusBadRequest, Reason: "invalid_job_id"}
	}
	err := c.do(ctx, http.MethodGet, &result, "jobs", id, "codex-result")
	return result, err
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
