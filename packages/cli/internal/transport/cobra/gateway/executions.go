package gatewaycmd

import (
	"context"
	"errors"
	"time"

	"github.com/spf13/cobra"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
	clierrors "github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/output"
)

func executionCommands() []*cobra.Command {
	workers := readCommand("workers", "gateway.cli.workers", func(ctx context.Context, client *gateway.Client) (any, error) {
		return client.ExecutionWorkers(ctx)
	})
	workers.AddCommand(readCommand("invite", "gateway.cli.worker_invite", func(ctx context.Context, client *gateway.Client) (any, error) {
		invite, err := client.InviteWorker(ctx)
		return invite, executionError(err)
	}))
	return []*cobra.Command{workers,
		readCommand("executions", "gateway.cli.executions", func(ctx context.Context, client *gateway.Client) (any, error) {
			return client.Executions(ctx)
		}),
		executionCommand(), waitExecutionCommand(), replyExecutionCommand(), continueExecutionCommand(), reviseExecutionCommand(), logsCommand()}
}

func reviseExecutionCommand() *cobra.Command {
	var comment string
	cmd := &cobra.Command{Use: "revise <job-id>", Args: i18n.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		client, url, err := openClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
		defer cancel()
		job, err := client.ReviseExecution(ctx, args[0], gateway.RevisionInput{CommentID: comment, Source: "cli"})
		if err != nil {
			return executionError(err)
		}
		emitExecution(url, job)
		return nil
	}}
	cmd.Flags().StringVar(&comment, "comment", "", "")
	i18n.MarkShort(cmd, "gateway.cli.revise")
	i18n.MarkFlagUsage(cmd, "comment", "gateway.cli.flag_revision_comment")
	return cmd
}

func emitExecution(url string, data any) {
	output.Emit(result{Schema: "one-cli/gateway/v1", URL: url, Data: data})
}

func executionCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "job <id>", Args: i18n.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		client, url, err := openClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
		defer cancel()
		job, err := client.Execution(ctx, args[0])
		if err != nil {
			return executionError(err)
		}
		emitExecution(url, job)
		return nil
	}}
	i18n.MarkShort(cmd, "gateway.cli.job")
	return cmd
}

func waitExecutionCommand() *cobra.Command {
	var timeout time.Duration
	cmd := &cobra.Command{Use: "wait <id>", Args: i18n.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if timeout <= 0 {
			return i18n.Errorf("gateway.cli.invalid_wait")
		}
		client, url, err := openClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
		defer cancel()
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			job, err := client.Execution(ctx, args[0])
			if ctx.Err() == context.DeadlineExceeded {
				return waitTimeout(args[0])
			}
			if err != nil {
				return executionError(err)
			}
			waitingToContinue := job.State == gateway.ExecutionWaitingToContinue && job.Progress != nil && !job.Progress.ContinueRequested
			if finished(job.State) || job.PendingQuestion() != nil || waitingToContinue {
				emitExecution(url, job)
				return nil
			}
			select {
			case <-ctx.Done():
				if ctx.Err() == context.DeadlineExceeded {
					return waitTimeout(args[0])
				}
				return ctx.Err()
			case <-ticker.C:
			}
		}
	}}
	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Minute, "")
	i18n.MarkShort(cmd, "gateway.cli.wait")
	i18n.MarkFlagUsage(cmd, "timeout", "gateway.cli.flag_wait_timeout")
	return cmd
}

func replyExecutionCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "reply <job-id> <question-id> <answer>", Args: i18n.ExactArgs(3), RunE: func(cmd *cobra.Command, args []string) error {
		client, url, err := openClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
		defer cancel()
		question, err := client.AnswerExecution(ctx, args[0], gateway.ExecutionAnswer{QuestionID: args[1], Answer: args[2]})
		if err != nil {
			return executionError(err)
		}
		emitExecution(url, question)
		return nil
	}}
	i18n.MarkShort(cmd, "gateway.cli.reply")
	return cmd
}

func continueExecutionCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "continue <job-id>", Args: i18n.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		client, url, err := openClient()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
		defer cancel()
		job, err := client.Execution(ctx, args[0])
		if err != nil {
			return executionError(err)
		}
		if job.Progress == nil || (job.State != gateway.ExecutionWaitingToContinue &&
			!(job.State == gateway.ExecutionQueued && job.Progress.ContinueRequested)) {
			return i18n.Errorf("gateway.execution_problem.execution_continue_conflict")
		}
		progress, err := client.ContinueExecution(ctx, job.ID, gateway.ContinueExecutionInput{ProgressID: job.Progress.ID})
		if err != nil {
			return executionError(err)
		}
		emitExecution(url, progress)
		return nil
	}}
	i18n.MarkShort(cmd, "gateway.cli.continue")
	return cmd
}

func logsCommand() *cobra.Command {
	var follow bool
	cmd := &cobra.Command{Use: "logs <id>", Args: i18n.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		client, _, err := openClient()
		if err != nil {
			return err
		}
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		ctx, offset := cmd.Context(), 0
		for {
			logs, err := client.ExecutionLogs(ctx, args[0], offset)
			if err != nil {
				return executionError(err)
			}
			if _, err := cmd.OutOrStdout().Write(logs.Data); err != nil {
				return err
			}
			offset = logs.Next
			if !follow || finished(logs.State) {
				if logs.Truncated {
					_, err := cmd.ErrOrStderr().Write([]byte(i18n.T("gateway.cli.logs_truncated") + "\n"))
					return err
				}
				return nil
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
			}
		}
	}}
	cmd.Flags().BoolVar(&follow, "follow", false, "")
	i18n.MarkShort(cmd, "gateway.cli.logs")
	i18n.MarkFlagUsage(cmd, "follow", "gateway.cli.flag_follow")
	return cmd
}

func finished(state string) bool {
	return state == gateway.ExecutionSucceeded || state == gateway.ExecutionFailed || state == gateway.ExecutionUnknown
}

func waitTimeout(id string) error {
	return clierrors.New(clierrors.ONE_CLI_ERROR, i18n.Tf("gateway.cli.wait_timeout", id)).
		WithContext(map[string]any{"reason": "wait_timeout", "jobId": id})
}

func executionError(err error) error {
	if err == nil {
		return nil
	}
	var response *gateway.ResponseError
	if errors.As(err, &response) {
		switch response.Reason {
		case "execution_question_conflict", "invalid_execution_answer", "execution_continue_conflict", "invalid_execution_continue",
			"invalid_revision", "revision_requires_open_pr", "revision_in_progress":
			return i18n.Errorf("gateway.execution_problem." + response.Reason)
		}
		return clierrors.New(clierrors.ONE_CLI_ERROR, i18n.Tf("gateway.cli.execution_response", response.Code)).
			WithContext(map[string]any{"reason": response.Reason}).WithCause(err)
	}
	return i18n.Errorf("gateway.cli.request_failed", err)
}
