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

// CodexOptions overrides the execution machine's defaults for one task.
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
	Issue string `json:"issue"`
	CodexOptions
}

type dispatchReply struct {
	Status Status `json:"status"`
}

func (c *Client) Dispatch(ctx context.Context, project, issue string) (Status, error) {
	var result dispatchReply
	err := c.do(ctx, http.MethodPost, &result, "projects", url.PathEscape(project), "jobs", url.PathEscape(issue))
	return result.Status, err
}

func (c *Client) DispatchWithOptions(ctx context.Context, project, issue string, options CodexOptions) (Status, error) {
	if options == (CodexOptions{}) {
		return c.Dispatch(ctx, project, issue)
	}
	if !options.Valid() {
		return "", ErrInvalidCodexOptions
	}
	var result dispatchReply
	input := DispatchInput{Issue: issue, CodexOptions: options}
	err := c.request(ctx, http.MethodPost, input, &result, "projects", url.PathEscape(project), "jobs")
	var problem *ResponseError
	if errors.As(err, &problem) {
		switch {
		case problem.Reason == "codex_options_conflict":
			return "", ErrCodexOptionsConflict
		case problem.Reason == "invalid_codex_options":
			return "", ErrInvalidCodexOptions
		case problem.Code == http.StatusMethodNotAllowed,
			problem.Code == http.StatusNotFound && problem.Reason == "",
			problem.Code == http.StatusBadRequest && problem.Reason == "invalid_mapping":
			return "", ErrCodexOptionsUnsupported
		}
	}
	return result.Status, err
}
