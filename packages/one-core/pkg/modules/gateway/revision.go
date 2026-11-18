package gateway

import (
	"context"
	"net/http"
)

type RevisionInput struct {
	CommentID string `json:"commentId"`
	Source    string `json:"source,omitempty"`
}

func (in RevisionInput) Valid() bool {
	return ValidIssueID(in.CommentID) && (in.Source == "" || in.Source == "cli" || in.Source == "desktop")
}

type LinearRevision struct {
	OriginID  string `json:"originId"`
	CommentID string `json:"commentId"`
}

func (c *Client) ReviseExecution(ctx context.Context, id string, input RevisionInput) (Execution, error) {
	var job Execution
	if !ValidExecutionID(id) || !input.Valid() {
		return job, &ResponseError{Code: http.StatusBadRequest, Reason: "invalid_revision"}
	}
	err := c.request(ctx, http.MethodPost, input, &job, "executions", id, "revise")
	return job, err
}
