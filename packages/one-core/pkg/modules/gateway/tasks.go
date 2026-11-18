package gateway

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"

	gitmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/git"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/knowledge"
)

type TaskFilter struct {
	Page     int
	WorkerID string
	Stage    string
	Source   string
}

func ParseTaskFilter(q url.Values) (TaskFilter, bool) {
	filter := TaskFilter{Page: 1, WorkerID: q.Get("worker"), Stage: q.Get("stage"), Source: q.Get("source")}
	if raw := q.Get("page"); raw != "" {
		var err error
		filter.Page, err = strconv.Atoi(raw)
		if err != nil || filter.Page < 1 {
			return TaskFilter{}, false
		}
	}
	return filter, (filter.WorkerID == "" || ValidWorkerID(filter.WorkerID)) &&
		(filter.Source == "" || filter.Source == "cli" || filter.Source == "desktop") && len(filter.Stage) <= 64
}

type TaskPage struct {
	Items     []Task `json:"items"`
	Page      int    `json:"page"`
	PageCount int    `json:"pageCount"`
	Partial   bool   `json:"partial,omitempty"`
}

type Task struct {
	Kind             string                   `json:"kind,omitempty"`
	OpenViking       *knowledge.SessionLink   `json:"openViking,omitempty"`
	ID               string                   `json:"id"`
	Identifier       string                   `json:"identifier,omitempty"`
	Title            string                   `json:"title,omitempty"`
	ProjectName      string                   `json:"projectName,omitempty"`
	Description      string                   `json:"description,omitempty"`
	URL              string                   `json:"url,omitempty"`
	Stage            string                   `json:"stage"`
	UpdatedAt        time.Time                `json:"updatedAt"`
	WorkerName       string                   `json:"workerName,omitempty"`
	LastError        string                   `json:"lastError,omitempty"`
	LocalResultJobID string                   `json:"localResultJobId,omitempty"`
	Codex            CodexOptions             `json:"codex,omitzero"`
	EffectiveCodex   CodexOptions             `json:"effectiveCodex,omitzero"`
	Repository       *gitmodule.RepositoryRef `json:"repository,omitempty"`
	Repositories     []JobRepository          `json:"repositories,omitempty"`
	PullRequests     []JobPullRequest         `json:"pullRequests,omitempty"`
	Subtasks         []SubtaskResult          `json:"subtasks,omitempty"`
	Runs             []TaskRun                `json:"runs,omitempty"`
}

type TaskRun struct {
	Codex          CodexOptions       `json:"codex,omitzero"`
	EffectiveCodex CodexOptions       `json:"effectiveCodex,omitzero"`
	ID             string             `json:"id"`
	WorkerID       string             `json:"workerId"`
	WorkerName     string             `json:"workerName"`
	Source         string             `json:"source"`
	State          string             `json:"state"`
	Revision       bool               `json:"revision,omitempty"`
	CreatedAt      time.Time          `json:"createdAt"`
	Question       *ExecutionQuestion `json:"question,omitempty"`
	Progress       *ExecutionProgress `json:"progress,omitempty"`
}

func (c *Client) Tasks(ctx context.Context, filter TaskFilter) (TaskPage, error) {
	var result TaskPage
	endpoint := c.base.JoinPath("v1", "tasks")
	endpoint.RawQuery = url.Values{
		"page": {strconv.Itoa(max(1, filter.Page))}, "worker": {filter.WorkerID},
		"stage": {filter.Stage}, "source": {filter.Source},
	}.Encode()
	err := c.send(ctx, http.MethodGet, endpoint, nil, &result)
	return result, err
}
