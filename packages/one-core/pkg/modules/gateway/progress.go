package gateway

import (
	"context"
	"net/http"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/knowledge"
)

const ExecutionWaitingToContinue = "waiting_to_continue"
const MaxExecutionProgress = 8 << 10

type ExecutionProgress struct {
	OpenViking        *knowledge.SessionLink `json:"openViking,omitempty"`
	ID                string                 `json:"id"`
	Round             int                    `json:"round"`
	Summary           string                 `json:"summary"`
	ContinueRequested bool                   `json:"continueRequested,omitempty"`
}

func (p ExecutionProgress) Valid() bool {
	return ValidProgressID(p.ID) && p.Round > 0 && validInteractionText(p.Summary, MaxExecutionProgress)
}

func ValidProgressID(id string) bool { return interactionID.MatchString(id) }

type ContinueExecutionInput struct {
	ProgressID string `json:"progressId"`
}

func (c *Client) ContinueExecution(ctx context.Context, id string, input ContinueExecutionInput) (ExecutionProgress, error) {
	var result ExecutionProgress
	if !ValidExecutionID(id) || !ValidProgressID(input.ProgressID) {
		return result, &ResponseError{Code: http.StatusBadRequest, Reason: "invalid_execution_continue"}
	}
	err := c.request(ctx, http.MethodPost, input, &result, "executions", id, "continue")
	return result, err
}
