package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	gitmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/git"
)

const (
	OneTask            = "one-task"
	CodexTask          = "codex"
	ExecutionQueued    = "queued"
	ExecutionRunning   = "running"
	ExecutionUnknown   = "outcome_unknown"
	ExecutionSucceeded = "succeeded"
	ExecutionFailed    = "failed"
)

const LinearWorkflow = "linear-workflow"

const MaxExecutionLogs = 256 << 10

type ExecutionWorker struct {
	LinearProjects []LinearProject           `json:"linearProjects,omitempty"`
	ID             string                    `json:"id"`
	Name           string                    `json:"name"`
	Online         bool                      `json:"online"`
	Busy           bool                      `json:"busy"`
	LastSeen       time.Time                 `json:"lastSeen,omitzero"`
	Repositories   []gitmodule.RepositoryRef `json:"repositories"`
	Models         *CodexModelCatalog        `json:"models,omitempty"`
}

type LinearProject struct {
	ID           string              `json:"id"`
	Repositories []RepositorySummary `json:"repositories"`
}

type ResolvedIssue struct {
	Dispatchable bool          `json:"dispatchable"`
	ID           string        `json:"id"`
	Identifier   string        `json:"identifier"`
	Title        string        `json:"title"`
	URL          string        `json:"url"`
	Project      LinearProject `json:"project"`
	LocalStatus  Status        `json:"localStatus,omitempty"`
}

type LinearExecution struct {
	Project    LinearProject `json:"project"`
	IssueID    string        `json:"issueId"`
	Identifier string        `json:"identifier"`
	Title      string        `json:"title"`
	URL        string        `json:"url"`
}

type ExecutionInput struct {
	Linear     *LinearExecution        `json:"linear,omitempty"`
	Kind       string                  `json:"kind"`
	Repository gitmodule.RepositoryRef `json:"repository"`
	Ref        string                  `json:"ref,omitempty"`
	CommitSHA  string                  `json:"commitSha,omitempty"`
	Project    string                  `json:"project,omitempty"`
	Task       string                  `json:"task,omitempty"`
	Prompt     string                  `json:"prompt,omitempty"`
	Codex      CodexOptions            `json:"codex,omitzero"`
	PublishRef bool                    `json:"publishRef,omitempty"`
}

var executionSelector = regexp.MustCompile(`^[a-zA-Z0-9_][a-zA-Z0-9_./:@-]{0,127}$`)
var executionID = regexp.MustCompile(`^[0-9a-f]{32}$`)
var commitSHA = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)

func ValidExecutionID(id string) bool { return executionID.MatchString(id) }

func ValidWorkerID(id string) bool { return executionSelector.MatchString(id) }

func (in ExecutionInput) Valid() bool {
	if in.Kind == LinearWorkflow {
		return in.Linear != nil && ValidIssueID(in.Linear.Project.ID) && ValidIssueID(in.Linear.IssueID) &&
			ValidIssueReference(in.Linear.Identifier) && in.Codex.Valid()
	}
	if in.Linear != nil {
		return false
	}
	if !in.Repository.Valid() || (in.Ref == "" && in.CommitSHA == "") ||
		(in.CommitSHA != "" && !commitSHA.MatchString(in.CommitSHA)) {
		return false
	}
	if in.Ref != "" && (!strings.HasPrefix(in.Ref, "refs/heads/") || !executionSelector.MatchString(in.Ref) || strings.Contains(in.Ref, "..")) {
		return false
	}
	switch in.Kind {
	case OneTask:
		return executionSelector.MatchString(in.Task) && executionSelector.MatchString(in.Project) &&
			!strings.Contains(in.Task, ":::") && in.Prompt == "" && in.Codex == (CodexOptions{}) && !in.PublishRef
	case CodexTask:
		return in.Project == "" && in.Task == "" && strings.TrimSpace(in.Prompt) != "" && len(in.Prompt) <= 24<<10 && in.Codex.Valid()
	}
	return false
}

type ExecutionSubmission struct {
	WorkerID  string         `json:"workerId"`
	RequestID string         `json:"requestId"`
	Source    string         `json:"source"`
	Input     ExecutionInput `json:"input"`
}

func (s ExecutionSubmission) Valid() bool {
	return executionSelector.MatchString(s.WorkerID) && executionSelector.MatchString(s.RequestID) &&
		(s.Source == "cli" || s.Source == "desktop") && s.Input.Valid()
}

type Execution struct {
	ID string `json:"id"`
	ExecutionSubmission
	State     string              `json:"state"`
	Reason    string              `json:"reason,omitempty"`
	CreatedAt time.Time           `json:"createdAt"`
	UpdatedAt time.Time           `json:"updatedAt"`
	Result    *ExecutionResult    `json:"result,omitempty"`
	Questions []ExecutionQuestion `json:"questions,omitempty"`
	Progress  *ExecutionProgress  `json:"progress,omitempty"`
}

type ExecutionResult struct {
	PullRequests  []JobPullRequest `json:"pullRequests,omitempty"`
	ExitCode      *int             `json:"exitCode,omitempty"`
	TaskResult    json.RawMessage  `json:"taskResult,omitempty"`
	CommitSHA     string           `json:"commitSha,omitempty"`
	Ref           string           `json:"ref,omitempty"`
	Response      string           `json:"response,omitempty"`
	ThreadID      string           `json:"threadId,omitempty"`
	Error         string           `json:"error,omitempty"`
	LogsTruncated bool             `json:"logsTruncated,omitempty"`
}

type ExecutionLogs struct {
	Data      []byte `json:"data"`
	Next      int    `json:"next"`
	State     string `json:"state"`
	Truncated bool   `json:"truncated"`
}

type WorkerInvite struct {
	URL       string    `json:"url"`
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expiresAt"`
}

func (c *Client) ExecutionWorkers(ctx context.Context) ([]ExecutionWorker, error) {
	var workers []ExecutionWorker
	err := c.do(ctx, http.MethodGet, &workers, "workers")
	return workers, err
}

func (c *Client) InviteWorker(ctx context.Context) (WorkerInvite, error) {
	var invite WorkerInvite
	err := c.do(ctx, http.MethodPost, &invite, "workers", "invite")
	return invite, err
}

func (c *Client) SubmitExecution(ctx context.Context, input ExecutionSubmission) (Execution, error) {
	var job Execution
	err := c.request(ctx, http.MethodPost, input, &job, "executions")
	return job, err
}

func (c *Client) Executions(ctx context.Context) ([]Execution, error) {
	var jobs []Execution
	err := c.do(ctx, http.MethodGet, &jobs, "executions")
	return jobs, err
}

func (c *Client) Execution(ctx context.Context, id string) (Execution, error) {
	var job Execution
	if !ValidExecutionID(id) {
		return job, &ResponseError{Code: http.StatusBadRequest, Reason: "invalid_execution_id"}
	}
	err := c.do(ctx, http.MethodGet, &job, "executions", id)
	return job, err
}

func (c *Client) ExecutionLogs(ctx context.Context, id string, offset int) (ExecutionLogs, error) {
	var logs ExecutionLogs
	if !ValidExecutionID(id) {
		return logs, &ResponseError{Code: http.StatusBadRequest, Reason: "invalid_execution_id"}
	}
	endpoint := c.base.JoinPath("v1", "executions", id, "logs")
	endpoint.RawQuery = "offset=" + strconv.Itoa(offset)
	err := c.send(ctx, http.MethodGet, endpoint, nil, &logs)
	return logs, err
}
