package serve

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	gateway "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
	tracking "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/issuetracking"
	cliErrors "github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

func registerGatewayRoutes(mux *http.ServeMux, opts MuxOpts, connect func() (*gateway.Client, error)) {
	mux.HandleFunc("GET /gateway/codex/models", func(w http.ResponseWriter, r *http.Request) {
		setNoStore(w)
		client, err := connect()
		if err != nil || client == nil {
			writeError(w, http.StatusServiceUnavailable, cliErrors.ONE_CLI_ERROR, i18n.T("gateway.codex_models_unavailable"), nil)
			return
		}
		catalog, err := client.CodexModels(r.Context())
		if err != nil {
			writeError(w, http.StatusBadGateway, cliErrors.ONE_CLI_ERROR, i18n.T("gateway.codex_models_unavailable"), nil)
			return
		}
		writeJSON(w, http.StatusOK, catalog)
	})
	mux.HandleFunc("GET /gateway/jobs/{job}/codex-result", func(w http.ResponseWriter, r *http.Request) {
		setNoStore(w)
		client, err := connect()
		if err != nil || client == nil {
			writeError(w, http.StatusServiceUnavailable, cliErrors.ONE_CLI_ERROR, i18n.T("gateway.unavailable"), nil)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		result, err := client.JobCodexResult(ctx, r.PathValue("job"))
		if err != nil {
			var response *gateway.ResponseError
			status := http.StatusBadGateway
			if errors.As(err, &response) {
				status = response.Code
			}
			writeError(w, status, cliErrors.ONE_CLI_ERROR, i18n.T("gateway.codex_result_unavailable"), nil)
			return
		}
		writeJSON(w, http.StatusOK, result)
	})
	mux.HandleFunc("GET /gateway/jobs", func(w http.ResponseWriter, r *http.Request) {
		setNoStore(w)
		page := 1
		if value := r.URL.Query().Get("page"); value != "" {
			var err error
			page, err = strconv.Atoi(value)
			if err != nil || page < 1 {
				writeError(w, http.StatusBadRequest, cliErrors.ONE_CLI_ERROR, i18n.T("gateway.jobs_invalid_page"), nil)
				return
			}
		}
		client, err := connect()
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, cliErrors.ONE_CLI_ERROR, i18n.T("gateway.connection_invalid"), nil)
			return
		}
		if client == nil {
			writeError(w, http.StatusServiceUnavailable, cliErrors.ONE_CLI_ERROR, i18n.T("gateway.unavailable"), nil)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
		defer cancel()
		jobs, err := client.MonitorJobs(ctx, page)
		if err != nil {
			var response *gateway.ResponseError
			if errors.As(err, &response) && response.Code == http.StatusNotFound {
				writeError(w, http.StatusNotFound, cliErrors.ONE_CLI_ERROR, i18n.T("gateway.jobs_unavailable"), nil)
				return
			}
			writeError(w, http.StatusBadGateway, cliErrors.ONE_CLI_ERROR, i18n.T("gateway.jobs_failed"), nil)
			return
		}
		writeJSON(w, http.StatusOK, jobs)
	})
	register := func(pattern string, dispatch bool) {
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			setNoStore(w)
			client, err := connect()
			if err != nil {
				writeError(w, http.StatusServiceUnavailable, cliErrors.ONE_CLI_ERROR, i18n.T("gateway.connection_invalid"), nil)
				return
			}
			if client == nil || opts.IssueTracking == nil {
				if dispatch {
					writeError(w, http.StatusServiceUnavailable, cliErrors.ONE_CLI_ERROR, i18n.T("gateway.unavailable"), nil)
				} else {
					writeJSON(w, http.StatusOK, gateway.Jobs{Issues: map[string]gateway.Status{}})
				}
				return
			}
			q := r.URL.Query()
			binding, err := opts.IssueTracking.Require(r.Context(), tracking.Scope{Kind: q.Get("kind"), ID: q.Get("scope")})
			if err != nil {
				writeIssueError(w, err)
				return
			}
			if dispatch {
				issue := r.PathValue("id")
				var options gateway.CodexOptions
				var workerID string
				if issue == "" {
					var input gateway.DispatchInput
					r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
					if err := decodeJSON(r, &input); err != nil || !gateway.ValidIssueReference(input.Issue) {
						writeError(w, http.StatusBadRequest, cliErrors.ONE_CLI_ERROR, i18n.T("gateway.codex_options_invalid"), nil)
						return
					}
					issue, options, workerID = input.Issue, input.CodexOptions, input.WorkerID
				}
				var result gateway.DispatchResult
				if workerID == "" {
					result.Status, err = client.DispatchWithOptions(r.Context(), binding.Project.ID, issue, options)
				} else {
					result, err = client.DispatchToWorker(r.Context(), binding.Project.ID, issue, options, workerID, "desktop")
				}
				var response *gateway.ResponseError
				switch {
				case errors.Is(err, gateway.ErrInvalidCodexOptions):
					writeError(w, http.StatusBadRequest, cliErrors.ONE_CLI_ERROR, i18n.T("gateway.codex_options_invalid"), nil)
				case errors.Is(err, gateway.ErrCodexOptionsConflict):
					writeError(w, http.StatusConflict, cliErrors.ONE_CLI_ERROR, i18n.T("gateway.codex_options_conflict"), nil)
				case errors.Is(err, gateway.ErrCodexOptionsUnsupported):
					writeError(w, http.StatusBadGateway, cliErrors.ONE_CLI_ERROR, i18n.T("gateway.codex_options_unsupported"), nil)
				case errors.As(err, &response) && response.Reason != "":
					writeError(w, response.Code, cliErrors.ONE_CLI_ERROR, gatewayExecutionError(response.Reason), map[string]any{"reason": response.Reason})
				case err != nil:
					writeError(w, http.StatusBadGateway, cliErrors.ONE_CLI_ERROR, i18n.Tf("gateway.dispatch_failed", err), nil)
				default:
					writeJSON(w, http.StatusAccepted, result)
				}
			} else {
				jobs, err := client.List(r.Context(), binding.Project.ID)
				if err != nil {
					writeError(w, http.StatusBadGateway, cliErrors.ONE_CLI_ERROR, i18n.Tf("gateway.status_failed", err), nil)
					return
				}
				writeJSON(w, http.StatusOK, jobs)
			}
		})
	}
	register("GET /issue-tracking/gateway/jobs", false)
	register("POST /issue-tracking/gateway/jobs", true)
	register("POST /issue-tracking/gateway/jobs/{id}", true)
}
