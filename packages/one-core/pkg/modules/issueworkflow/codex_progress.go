package issueworkflow

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"strings"

	api "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

const turnOutcomeInstructions = "Return a final JSON object with completed and summary. Set completed to true only if all requested work is done. If this turn stops partway, set completed to false, even if you know how to continue. In summary, describe what changed, what remains, and any blocker. Honor any stopping point specified by the requester. Write the summary in the issue's language."

func turnOutcomeSchema() map[string]any {
	return map[string]any{
		"type": "object", "additionalProperties": false,
		"required": []string{"completed", "summary"},
		"properties": map[string]any{
			"completed": map[string]string{"type": "boolean"},
			"summary":   map[string]string{"type": "string"},
		},
	}
}

func continuePrompt(progress api.ExecutionProgress) string {
	return "Continue the original Linear issue in the existing worktree, starting from the progress below. Inspect the existing changes before editing. If the next step was unknown, investigate within the original task scope.\n\nPrevious progress:\n" + progress.Summary
}

func recordTurnOutcome(job *Job, status string, message finalCodexMessage, checkpoint func(Job) error) error {
	if !job.TrackProgress {
		if status != "completed" {
			return fmt.Errorf("Codex turn ended with status %s", status)
		}
		job.CodexResponse = message.text
		return nil
	}
	var outcome struct {
		Completed *bool  `json:"completed"`
		Summary   string `json:"summary"`
	}
	valid := json.Unmarshal([]byte(message.text), &outcome) == nil && outcome.Completed != nil && strings.TrimSpace(outcome.Summary) != ""
	if status == "completed" && valid && *outcome.Completed {
		job.CodexResponse = strings.TrimSpace(outcome.Summary)
		return nil
	}
	summary := message.text
	if valid {
		summary = outcome.Summary
	} else if summary == "" {
		summary = message.latest
	}
	if strings.TrimSpace(summary) == "" {
		if status == "completed" {
			summary = i18n.T("gateway.workflow_progress_unconfirmed")
		} else {
			summary = i18n.T("gateway.workflow_progress_interrupted")
		}
	}
	return saveExecutionProgress(job, summary, checkpoint)
}

func saveExecutionProgress(job *Job, summary string, checkpoint func(Job) error) error {
	summary = strings.TrimSpace(strings.ReplaceAll(summary, "\x00", ""))
	summary = strings.ToValidUTF8(summary[:min(len(summary), api.MaxExecutionProgress)], "")
	if summary == "" {
		summary = i18n.T("gateway.workflow_progress_unconfirmed")
	}
	updated := *job
	round := 1
	if job.Progress != nil {
		round = job.Progress.Round + 1
	}
	updated.Progress = &api.ExecutionProgress{ID: rand.Text(), Round: round, Summary: summary}
	updated.Stage, updated.LastError, updated.CodexResponse = waitingToContinue, "", ""
	if err := checkpoint(updated); err != nil {
		return err
	}
	*job = updated
	return nil
}
