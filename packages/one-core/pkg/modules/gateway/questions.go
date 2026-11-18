package gateway

import (
	"context"
	"net/http"
	"regexp"
	"strings"
)

const ExecutionWaitingForAnswer = "waiting_for_answer"

const MaxQuestionBytes = 8 << 10
const MaxAnswerBytes = 8 << 10

var interactionID = regexp.MustCompile(`^[A-Z2-7]{26}$`)

type ExecutionQuestion struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	Answer string `json:"answer,omitempty"`
}

func (q ExecutionQuestion) Valid() bool {
	return interactionID.MatchString(q.ID) && validInteractionText(q.Text, MaxQuestionBytes)
}

type ExecutionAnswer struct {
	QuestionID string `json:"questionId"`
	Answer     string `json:"answer"`
}

func (a ExecutionAnswer) Valid() bool {
	return interactionID.MatchString(a.QuestionID) && validInteractionText(a.Answer, MaxAnswerBytes)
}

func validInteractionText(text string, limit int) bool {
	return len(text) <= limit && strings.TrimSpace(text) != "" && !strings.ContainsRune(text, '\x00')
}

func (e Execution) PendingQuestion() *ExecutionQuestion {
	if e.State != ExecutionWaitingForAnswer || len(e.Questions) == 0 {
		return nil
	}
	question := e.Questions[len(e.Questions)-1]
	if question.Answer != "" {
		return nil
	}
	return &question
}

func (c *Client) AnswerExecution(ctx context.Context, id string, input ExecutionAnswer) (ExecutionQuestion, error) {
	var result ExecutionQuestion
	if !ValidExecutionID(id) || !input.Valid() {
		return result, &ResponseError{Code: http.StatusBadRequest, Reason: "invalid_execution_answer"}
	}
	err := c.request(ctx, http.MethodPost, input, &result, "executions", id, "answer")
	return result, err
}
