package gateway

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"unicode"
)

var (
	ErrInvalidCodexOptions     = errors.New("invalid Codex execution options")
	ErrCodexOptionsConflict    = errors.New("task already dispatched with different Codex execution options")
	ErrCodexOptionsUnsupported = errors.New("Gateway does not support Codex execution options")
)

// CodexOptions describes a model and reasoning effort. Dispatch can omit either
// field to use the execution machine's default.
type CodexOptions struct {
	Model  string `json:"model,omitempty"`
	Effort string `json:"effort,omitempty"`
}

func (o CodexOptions) Valid() bool {
	return validSelector(o.Model, 128) && validSelector(o.Effort, 32)
}

func validSelector(value string, maxBytes int) bool {
	return len(value) <= maxBytes && !strings.ContainsFunc(value, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r)
	})
}

type DispatchInput struct {
	Issue        string   `json:"issue"`
	SubtaskOrder []string `json:"subtaskOrder,omitempty"`
	WorkerID     string   `json:"workerId,omitempty"`
	Source       string   `json:"source,omitempty"`
	CodexOptions
}

func ValidSubtaskOrder(order []string) bool {
	seen := make(map[string]bool, len(order))
	for _, ref := range order {
		key := strings.ToLower(ref)
		if !ValidIssueReference(ref) || seen[key] {
			return false
		}
		seen[key] = true
	}
	return true
}

type DispatchResult struct {
	Status    Status `json:"status"`
	JobID     string `json:"jobId,omitempty"`
	ProjectID string `json:"projectId,omitempty"`
}

func (c *Client) DispatchIssue(ctx context.Context, input DispatchInput) (DispatchResult, error) {
	var result DispatchResult
	err := c.request(ctx, http.MethodPost, input, &result, "jobs")
	return result, err
}

func (c *Client) Dispatch(ctx context.Context, project, issue string) (Status, error) {
	var result DispatchResult
	err := c.do(ctx, http.MethodPost, &result, "projects", url.PathEscape(project), "jobs", url.PathEscape(issue))
	return result.Status, err
}

func (c *Client) DispatchWithOptions(ctx context.Context, project, issue string, options CodexOptions) (Status, error) {
	if options == (CodexOptions{}) {
		return c.Dispatch(ctx, project, issue)
	}
	result, err := c.DispatchToWorker(ctx, project, issue, options, "", "")
	return result.Status, err
}

func (c *Client) DispatchToWorker(ctx context.Context, project, issue string, options CodexOptions, workerID, source string) (DispatchResult, error) {
	if !options.Valid() {
		return DispatchResult{}, ErrInvalidCodexOptions
	}
	var result DispatchResult
	input := DispatchInput{Issue: issue, CodexOptions: options, WorkerID: workerID, Source: source}
	err := c.request(ctx, http.MethodPost, input, &result, "projects", url.PathEscape(project), "jobs")
	var problem *ResponseError
	if errors.As(err, &problem) {
		switch {
		case problem.Reason == "codex_options_conflict":
			return result, ErrCodexOptionsConflict
		case problem.Reason == "invalid_codex_options":
			return result, ErrInvalidCodexOptions
		case problem.Code == http.StatusMethodNotAllowed,
			problem.Code == http.StatusNotFound && problem.Reason == "",
			problem.Code == http.StatusBadRequest && problem.Reason == "invalid_mapping":
			return result, ErrCodexOptionsUnsupported
		}
	}
	return result, err
}
