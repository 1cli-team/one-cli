package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"time"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/codehost"
	gitmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/git"
	tracking "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/issuetracking"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/knowledge"
)

const (
	SetupStatus       = "status"
	SetupGitea        = "gitea"
	SetupOpenViking   = "openviking"
	SetupClone        = "clone"
	SetupProject      = "project"
	SetupLinearStart  = "linear-start"
	SetupLinearFinish = "linear-finish"
	SetupLinearCancel = "linear-cancel"
)

type SetupInput struct {
	Operation  string                     `json:"operation"`
	Gitea      *codehost.Input            `json:"gitea,omitempty"`
	OpenViking *knowledge.ConnectionInput `json:"openviking,omitempty"`
	OAuth      *tracking.OAuthConfig      `json:"oauth,omitempty"`
	State      string                     `json:"state,omitempty"`
	Code       string                     `json:"code,omitempty"`
	Remote     string                     `json:"remote,omitempty"`
	Project    *SetupMapping              `json:"project,omitempty"`
}

type SetupMapping struct {
	ID           string                    `json:"id"`
	Connection   string                    `json:"connectionId,omitempty"`
	Organization string                    `json:"organizationId"`
	Repositories []gitmodule.RepositoryRef `json:"repositories"`
	Base         string                    `json:"base,omitempty"`
	CheckTask    string                    `json:"checkTask,omitempty"`
	Teams        map[string]TeamFlow       `json:"teams"`
}

type SetupReadiness struct {
	Linear    []tracking.Connection `json:"linear"`
	CodeHosts []codehost.Connection `json:"codeHosts"`
	Workflow  bool                  `json:"workflow"`
	Projects  []ProjectSummary      `json:"projects"`
}

type SetupCommand struct {
	ID       string     `json:"id"`
	Input    SetupInput `json:"input"`
	Deadline time.Time  `json:"deadline"`
}

var setupWorkerID = regexp.MustCompile(`^(local|w-[a-z2-7]{26})$`)

var ErrInvalidSetupWorker = errors.New("invalid setup worker ID")

func (c *Client) Setup(ctx context.Context, worker string, input SetupInput, out any) error {
	if !setupWorkerID.MatchString(worker) {
		return ErrInvalidSetupWorker
	}
	return c.request(ctx, http.MethodPost, input, out, "workers", worker, "setup")
}

// SetupResult carries the target machine’s response.
type SetupResult struct {
	ID    string    `json:"id"`
	Reply WorkReply `json:"reply"`
}

func SetupReply(status int, value any) WorkReply {
	body, err := json.Marshal(value)
	if err != nil {
		return WorkReply{Status: http.StatusInternalServerError, Body: json.RawMessage(`{"reason":"setup_response"}`)}
	}
	return WorkReply{Status: status, Body: body}
}
