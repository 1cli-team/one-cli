package issueworkflow

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	api "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
)

const askUserTool = "one_gateway_ask_user"

var errCodexWaitingForAnswer = errors.New("waiting for the user's answer")

func questionToolDefinition() map[string]any {
	return map[string]any{
		"type": "function", "name": askUserTool,
		"description": "Ask the requester a question when a missing requirement or product decision blocks implementation. This pauses the workflow until the requester answers. Never request command, sandbox, or network permissions here.",
		"inputSchema": map[string]any{
			"type": "object", "additionalProperties": false, "required": []string{"question"},
			"properties": map[string]any{
				"question": map[string]string{"type": "string", "description": "The question in the issue's language. Include the relevant options or tradeoff when helpful."},
			},
		},
	}
}

func (s *codexSession) captureQuestion(job *Job, checkpoint func(Job) error, frame rpcFrame) (bool, error) {
	if frame.Method != "item/tool/call" {
		return false, nil
	}
	var event struct {
		ThreadID  string `json:"threadId"`
		TurnID    string `json:"turnId"`
		Tool      string `json:"tool"`
		Arguments struct {
			Question string `json:"question"`
		} `json:"arguments"`
	}
	if json.Unmarshal(frame.Params, &event) != nil || event.Tool != askUserTool ||
		event.ThreadID != job.ThreadID || event.TurnID == "" || (job.TurnID != "" && event.TurnID != job.TurnID) {
		return false, nil
	}
	question := api.ExecutionQuestion{ID: rand.Text(), Text: strings.TrimSpace(event.Arguments.Question)}
	if !question.Valid() {
		return true, s.in.Encode(map[string]any{"id": frame.ID, "result": map[string]any{
			"success": false, "contentItems": []map[string]string{{
				"type": "inputText", "text": "Provide a non-empty, concise question.",
			}},
		}})
	}
	updated := *job
	updated.TurnID, updated.Question, updated.Stage = event.TurnID, &question, waitingForAnswer
	if err := checkpoint(updated); err != nil {
		return true, err
	}
	*job = updated
	return true, errCodexWaitingForAnswer
}

func (s *codexSession) interruptTurn(threadID, turnID string) {
	s.handleRequest = nil
	finished := false
	onFrame := func(frame rpcFrame) {
		if frame.Method != "turn/completed" {
			return
		}
		var event struct {
			ThreadID string    `json:"threadId"`
			Turn     codexTurn `json:"turn"`
		}
		finished = json.Unmarshal(frame.Params, &event) == nil && event.ThreadID == threadID && event.Turn.ID == turnID
	}
	if s.call("turn/interrupt", map[string]string{"threadId": threadID, "turnId": turnID}, nil, onFrame) != nil {
		return
	}
	for !finished {
		frame, err := s.read()
		if err != nil {
			return
		}
		onFrame(frame)
	}
}

func answerPrompt(question api.ExecutionQuestion) string {
	return fmt.Sprintf("The requester answered the question that paused this task. Continue implementing the original Linear issue in the current worktree using the answer below.\n\nQuestion:\n%s\n\nAnswer:\n%s", question.Text, question.Answer)
}
