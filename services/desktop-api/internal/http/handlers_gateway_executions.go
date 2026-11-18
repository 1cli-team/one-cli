package serve

import (
	"context"
	"errors"
	"net/http"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/knowledge"
	clierrors "github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

func registerGatewayExecutionRoutes(mux *http.ServeMux, connect func() (*gateway.Client, error), backend knowledge.Backend) {
	if source, ok := backend.(interface {
		ConnectionForSync(context.Context) (knowledge.ConnectionInput, error)
	}); ok {
		registerGatewayOpenVikingSyncRoute(mux, source.ConnectionForSync, gateway.ReadConnection)
	}
	handle := func(pattern string, status int, call func(http.ResponseWriter, *http.Request, *gateway.Client) (any, error)) {
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			setNoStore(w)
			client, err := connect()
			if err != nil || client == nil {
				writeError(w, http.StatusServiceUnavailable, clierrors.ONE_CLI_ERROR, i18n.T("gateway.unavailable"), nil)
				return
			}
			value, err := call(w, r, client)
			if err != nil {
				var problem *gateway.ResponseError
				if errors.As(err, &problem) && problem.Code >= 400 && problem.Code <= 599 {
					writeError(w, problem.Code, clierrors.ONE_CLI_ERROR, gatewayExecutionError(problem.Reason), map[string]any{"reason": problem.Reason})
				} else {
					writeError(w, http.StatusBadGateway, clierrors.ONE_CLI_ERROR, i18n.Tf("gateway.status_failed", err), nil)
				}
				return
			}
			writeJSON(w, status, value)
		})
	}
	handle("GET /gateway/workers", http.StatusOK, func(_ http.ResponseWriter, r *http.Request, client *gateway.Client) (any, error) {
		return client.ExecutionWorkers(r.Context())
	})
	handle("GET /gateway/settings/downloads", http.StatusOK, func(_ http.ResponseWriter, r *http.Request, client *gateway.Client) (any, error) {
		return client.DownloadSettings(r.Context())
	})
	handle("PUT /gateway/settings/downloads", http.StatusOK, func(w http.ResponseWriter, r *http.Request, client *gateway.Client) (any, error) {
		var input gateway.DownloadSettingsInput
		r.Body = http.MaxBytesReader(w, r.Body, 1024)
		if err := decodeJSON(r, &input); err != nil || input.ChinaMirror == nil {
			return nil, &gateway.ResponseError{Code: http.StatusBadRequest, Reason: "invalid_download_settings"}
		}
		return client.SetDownloadSettings(r.Context(), input)
	})
	handle("GET /gateway/tasks", http.StatusOK, func(_ http.ResponseWriter, r *http.Request, client *gateway.Client) (any, error) {
		filter, valid := gateway.ParseTaskFilter(r.URL.Query())
		if !valid {
			return nil, &gateway.ResponseError{Code: http.StatusBadRequest, Reason: "invalid_task_filter"}
		}
		return client.Tasks(r.Context(), filter)
	})
	handle("POST /gateway/workers/invite", http.StatusOK, func(_ http.ResponseWriter, r *http.Request, client *gateway.Client) (any, error) {
		return client.InviteWorker(r.Context())
	})
	handle("GET /gateway/executions", http.StatusOK, func(_ http.ResponseWriter, r *http.Request, client *gateway.Client) (any, error) {
		return client.Executions(r.Context())
	})
	handle("GET /gateway/executions/{id}", http.StatusOK, func(_ http.ResponseWriter, r *http.Request, client *gateway.Client) (any, error) {
		return client.Execution(r.Context(), r.PathValue("id"))
	})
	handle("POST /gateway/executions/{id}/answer", http.StatusOK, func(w http.ResponseWriter, r *http.Request, client *gateway.Client) (any, error) {
		var input gateway.ExecutionAnswer
		r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
		if err := decodeJSON(r, &input); err != nil {
			return nil, &gateway.ResponseError{Code: http.StatusBadRequest, Reason: "invalid_execution_answer"}
		}
		return client.AnswerExecution(r.Context(), r.PathValue("id"), input)
	})
	handle("POST /gateway/executions/{id}/continue", http.StatusOK, func(w http.ResponseWriter, r *http.Request, client *gateway.Client) (any, error) {
		var input gateway.ContinueExecutionInput
		r.Body = http.MaxBytesReader(w, r.Body, 1024)
		if err := decodeJSON(r, &input); err != nil {
			return nil, &gateway.ResponseError{Code: http.StatusBadRequest, Reason: "invalid_execution_continue"}
		}
		return client.ContinueExecution(r.Context(), r.PathValue("id"), input)
	})
	handle("POST /gateway/executions/{id}/revise", http.StatusAccepted, func(w http.ResponseWriter, r *http.Request, client *gateway.Client) (any, error) {
		var input gateway.RevisionInput
		r.Body = http.MaxBytesReader(w, r.Body, 1024)
		if err := decodeJSON(r, &input); err != nil {
			return nil, &gateway.ResponseError{Code: http.StatusBadRequest, Reason: "invalid_revision"}
		}
		input.Source = "desktop"
		return client.ReviseExecution(r.Context(), r.PathValue("id"), input)
	})
	handle("GET /gateway/executions/{id}/logs", http.StatusOK, func(_ http.ResponseWriter, r *http.Request, client *gateway.Client) (any, error) {
		return client.ExecutionLogs(r.Context(), r.PathValue("id"), 0)
	})
}

func gatewayExecutionError(reason string) string {
	switch reason {
	case "worker_not_found", "worker_repository_unavailable", "worker_kind_unavailable", "worker_model_unavailable",
		"worker_mirror_unavailable", "invalid_download_settings",
		"execution_conflict", "execution_not_found", "invalid_execution", "invalid_execution_id",
		"issue_worker_conflict", "worker_workflow_unavailable", "workflow_dispatch", "invalid_issue_reference", "issue_ineligible",
		"execution_question_conflict", "invalid_execution_answer", "execution_continue_conflict", "invalid_execution_continue",
		"invalid_subtask_order", "subtask_order_conflict", "subtasks_require_single_repository", "subtask_parent_unavailable",
		"invalid_revision", "revision_requires_open_pr", "revision_in_progress", "invalid_task_filter":
		return i18n.T("gateway.execution_problem." + reason)
	default:
		return i18n.T("gateway.execution_problem.request")
	}
}
