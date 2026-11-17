package gatewaycmd

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/output"
)

func dispatchCommand() *cobra.Command {
	var worker, model, effort string
	cmd := &cobra.Command{
		Use:  "dispatch <issue>",
		Args: i18n.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			issue := strings.TrimSpace(args[0])
			if !gateway.ValidIssueReference(issue) {
				return i18n.Errorf("gateway.cli.invalid_issue")
			}
			workerID := strings.TrimSpace(worker)
			if workerID == "" {
				return i18n.Errorf("gateway.execution_problem.worker_required")
			}
			if !gateway.ValidWorkerID(workerID) {
				return i18n.Errorf("gateway.execution_problem.worker_not_found")
			}
			options := gateway.CodexOptions{Model: strings.TrimSpace(model), Effort: strings.TrimSpace(effort)}
			if !options.Valid() {
				return i18n.Errorf("gateway.codex_options_invalid")
			}
			client, url, err := openClient()
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 45*time.Second)
			defer cancel()
			dispatch, err := client.DispatchIssue(ctx, gateway.DispatchInput{
				Issue: issue, WorkerID: workerID, Source: "cli", CodexOptions: options,
			})
			if err != nil {
				var response *gateway.ResponseError
				if errors.As(err, &response) {
					switch response.Reason {
					case "codex_options_conflict":
						return i18n.Errorf("gateway.codex_options_conflict")
					case "issue_unmapped", "issue_reference_ambiguous", "issue_worker_conflict", "issue_ineligible",
						"worker_workflow_unavailable", "worker_not_found", "worker_model_unavailable", "workflow_dispatch":
						return i18n.Errorf("gateway.execution_problem." + response.Reason)
					case "":
						if response.Code == http.StatusNotFound || response.Code == http.StatusMethodNotAllowed {
							return i18n.Errorf("gateway.cli.issue_dispatch_unsupported")
						}
					}
				}
				return executionError(err)
			}
			output.Emit(result{
				Schema: "one-cli/gateway/v1", URL: url,
				Data: dispatchResult{Issue: issue, DispatchResult: dispatch},
			})
			return nil
		},
	}
	i18n.MarkShort(cmd, "gateway.cli.dispatch")
	i18n.MarkLong(cmd, "gateway.cli.dispatch_long")
	cmd.Flags().StringVar(&worker, "worker", "", "")
	cmd.Flags().StringVar(&model, "model", "", "")
	cmd.Flags().StringVar(&effort, "effort", "", "")
	i18n.MarkFlagUsage(cmd, "worker", "gateway.cli.flag_worker")
	i18n.MarkFlagUsage(cmd, "model", "gateway.cli.flag_model")
	i18n.MarkFlagUsage(cmd, "effort", "gateway.cli.flag_effort")
	return cmd
}

type dispatchResult struct {
	Issue string `json:"issue"`
	gateway.DispatchResult
}
