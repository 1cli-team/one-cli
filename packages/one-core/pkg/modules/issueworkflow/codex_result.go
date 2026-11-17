package issueworkflow

import (
	"context"
	"os"
	"time"

	api "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
)

func (j Job) canReadCodexResult() bool {
	if j.CodexResponse != "" {
		return true
	}
	switch j.Stage {
	case verifying, publishing, opening, toReview, review, toDone, done:
		return j.ThreadID != "" && j.TurnID != ""
	default:
		return false
	}
}

func (s *Service) codexJobResult(ctx context.Context, job Job) (api.CodexJobResult, error) {
	if job.CodexResponse != "" || !job.canReadCodexResult() {
		return api.CodexJobResult{Response: job.CodexResponse}, nil
	}
	reader, ok := s.runner.(interface {
		CompletedResponse(context.Context, string, string) (string, error)
	})
	if !ok {
		return api.CodexJobResult{}, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	text, err := reader.CompletedResponse(ctx, job.ThreadID, job.TurnID)
	return api.CodexJobResult{Response: text}, err
}

// CompletedResponse reads only the recorded turn. It can use a temporary
// directory because historical results do not depend on a retained worktree.
func (c Codex) CompletedResponse(ctx context.Context, threadID, turnID string) (string, error) {
	tmp, err := os.MkdirTemp(c.StateDir, "codex-result-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	session, err := c.start(ctx, tmp, "", tmp)
	if err != nil {
		return "", err
	}
	defer session.stop()
	params := map[string]any{
		"threadId": threadID, "limit": 1, "sortDirection": "desc", "itemsView": "full",
	}
	for {
		var page struct {
			Data       []codexTurn `json:"data"`
			NextCursor string      `json:"nextCursor"`
		}
		if err := session.call("thread/turns/list", params, &page, nil); err != nil {
			return "", err
		}
		for _, turn := range page.Data {
			if turn.ID == turnID {
				if turn.Status != "completed" {
					return "", nil
				}
				var message finalCodexMessage
				for _, item := range turn.Items {
					message.record(item)
				}
				return message.text, nil
			}
		}
		if page.NextCursor == "" || page.NextCursor == params["cursor"] {
			return "", nil
		}
		params["cursor"] = page.NextCursor
	}
}
