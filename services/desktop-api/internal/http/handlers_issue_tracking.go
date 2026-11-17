package serve

import (
	"errors"
	"net/http"
	"strconv"

	tracking "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/issuetracking"
	cliErrors "github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/errors"
)

func registerIssueTrackingRoutes(mux *http.ServeMux, opts MuxOpts) {
	service := opts.IssueTracking
	register := func(pattern string, handler func(*http.Request) (any, error)) {
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			setNoStore(w)
			if service == nil {
				writeIssueError(w, tracking.Fail("unavailable"))
				return
			}
			result, err := handler(r)
			if err != nil {
				writeIssueError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, result)
		})
	}
	scope := func(r *http.Request) tracking.Scope {
		return tracking.Scope{Kind: r.URL.Query().Get("kind"), ID: r.URL.Query().Get("scope")}
	}
	register("GET /linear/connections", func(r *http.Request) (any, error) { return service.Backend.Status(r.Context()) })
	register("PUT /linear/config", func(r *http.Request) (any, error) {
		var input tracking.OAuthConfig
		if err := decodeJSON(r, &input); err != nil {
			return nil, tracking.Fail("input")
		}
		if err := service.Backend.Configure(r.Context(), input); err != nil {
			return nil, err
		}
		return service.Backend.Status(r.Context())
	})
	register("POST /linear/authorize", func(r *http.Request) (any, error) { return service.Backend.Authorize(opts.Lifecycle) })
	register("DELETE /linear/authorize", func(r *http.Request) (any, error) {
		service.Backend.CancelAuthorization()
		return service.Backend.Status(r.Context())
	})
	register("DELETE /linear/connections/{id}", func(r *http.Request) (any, error) {
		if err := service.Backend.Disconnect(r.Context(), r.PathValue("id")); err != nil {
			return nil, err
		}
		return service.Backend.Status(r.Context())
	})
	register("GET /issue-tracking/projects", func(r *http.Request) (any, error) {
		return service.Backend.Projects(r.Context(), r.URL.Query().Get("connection"), r.URL.Query().Get("after"))
	})
	register("GET /issue-tracking/projects/{id}", func(r *http.Request) (any, error) {
		return service.Backend.Project(r.Context(), r.URL.Query().Get("connection"), r.PathValue("id"))
	})
	register("GET /issue-tracking/teams/{id}", func(r *http.Request) (any, error) {
		return service.Backend.Team(r.Context(), r.URL.Query().Get("connection"), r.PathValue("id"))
	})
	register("GET /issue-tracking/binding", func(r *http.Request) (any, error) {
		binding, err := service.Binding(r.Context(), scope(r))
		return map[string]any{"binding": binding}, err
	})
	register("PUT /issue-tracking/binding", func(r *http.Request) (any, error) {
		var input tracking.Binding
		if err := decodeJSON(r, &input); err != nil {
			return nil, tracking.Fail("input")
		}
		input.Scope = scope(r)
		return service.Bind(r.Context(), input)
	})
	register("DELETE /issue-tracking/binding", func(r *http.Request) (any, error) {
		return map[string]bool{"unbound": true}, service.Unbind(r.Context(), scope(r))
	})
	register("GET /issue-tracking/issues", func(r *http.Request) (any, error) {
		binding, err := service.Require(r.Context(), scope(r))
		if err != nil {
			return nil, err
		}
		q := r.URL.Query()
		filter := tracking.IssueFilter{After: q.Get("after"), Query: q.Get("q"), StateID: q.Get("state"), TeamID: q.Get("team"), AssigneeID: q.Get("assignee")}
		if q.Has("priority") {
			priority, err := strconv.Atoi(q.Get("priority"))
			if err != nil {
				return nil, tracking.Fail("input")
			}
			filter.Priority = &priority
		}
		return service.Backend.Issues(r.Context(), binding, filter)
	})
	register("GET /issue-tracking/issues/{id}", func(r *http.Request) (any, error) {
		binding, err := service.Require(r.Context(), scope(r))
		if err != nil {
			return nil, err
		}
		return service.Backend.Issue(r.Context(), binding, r.PathValue("id"))
	})
	register("POST /issue-tracking/issues", func(r *http.Request) (any, error) {
		binding, err := service.Require(r.Context(), scope(r))
		if err != nil {
			return nil, err
		}
		var input tracking.IssueInput
		if err := decodeJSON(r, &input); err != nil {
			return nil, tracking.Fail("input")
		}
		return service.Backend.CreateIssue(r.Context(), binding, input)
	})
	register("PUT /issue-tracking/issues/{id}", func(r *http.Request) (any, error) {
		binding, err := service.Require(r.Context(), scope(r))
		if err != nil {
			return nil, err
		}
		var input tracking.IssueInput
		if err := decodeJSON(r, &input); err != nil {
			return nil, tracking.Fail("input")
		}
		return service.Backend.UpdateIssue(r.Context(), binding, r.PathValue("id"), input)
	})
	register("GET /issue-tracking/issues/{id}/comments", func(r *http.Request) (any, error) {
		binding, err := service.Require(r.Context(), scope(r))
		if err != nil {
			return nil, err
		}
		nodes, page, err := service.Backend.Comments(r.Context(), binding, r.PathValue("id"), r.URL.Query().Get("after"))
		return map[string]any{"nodes": nodes, "pageInfo": page}, err
	})
	register("POST /issue-tracking/issues/{id}/comments", func(r *http.Request) (any, error) {
		binding, err := service.Require(r.Context(), scope(r))
		if err != nil {
			return nil, err
		}
		var input struct {
			Body        string `json:"body"`
			OperationID string `json:"operationId"`
		}
		if err := decodeJSON(r, &input); err != nil {
			return nil, tracking.Fail("input")
		}
		return service.Backend.Comment(r.Context(), binding, r.PathValue("id"), input.Body, input.OperationID)
	})
	register("GET /issue-tracking/operations/{id}", func(r *http.Request) (any, error) {
		binding, err := service.Require(r.Context(), scope(r))
		if err != nil {
			return nil, err
		}
		return service.Backend.Operation(r.Context(), binding, r.PathValue("id"))
	})
	register("GET /issue-tracking/operations", func(r *http.Request) (any, error) {
		binding, err := service.Require(r.Context(), scope(r))
		if err != nil {
			return nil, err
		}
		return service.Backend.PendingOperations(r.Context(), binding)
	})
}

func writeIssueError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	context := map[string]any{}
	var problem *tracking.Error
	if errors.As(err, &problem) {
		context["linearCode"] = problem.Code
		if problem.OperationID != "" {
			context["operationId"] = problem.OperationID
		}
		switch problem.Code {
		case "input", "config", "team", "operation", "oauth_state":
			status = http.StatusBadRequest
		case "scope", "not_found":
			status = http.StatusNotFound
		case "connection", "reconnect":
			status = http.StatusUnauthorized
		case "forbidden":
			status = http.StatusForbidden
		case "rate_limit":
			status = http.StatusTooManyRequests
		case "binding", "busy", "conflict", "outcome_unknown", "callback_busy":
			status = http.StatusConflict
		case "unavailable":
			status = http.StatusServiceUnavailable
		case "network":
			status = http.StatusBadGateway
		}
	}
	writeError(w, status, cliErrors.ONE_CLI_ERROR, err.Error(), context)
}
