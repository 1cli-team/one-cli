package serve

import (
	"errors"
	"net/http"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/workspacegroups"
	cliErrors "github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

func registerGroupRoutes(mux *http.ServeMux, opts MuxOpts) {
	// The route wrapper handles unavailable compositions and shares the domain
	// error mapping across read and mutation endpoints.
	register := func(pattern string, status int, handler func(*http.Request) (any, error)) {
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			if opts.GroupService == nil {
				writeError(w, http.StatusServiceUnavailable, cliErrors.ONE_CLI_ERROR, i18n.T("groups.unavailable"), nil)
				return
			}
			result, err := handler(r)
			if err != nil {
				writeGroupError(w, err)
				return
			}
			if status == http.StatusNoContent {
				w.WriteHeader(status)
				return
			}
			writeJSON(w, status, result)
		})
	}
	register("GET /workspace-groups", http.StatusOK, func(r *http.Request) (any, error) {
		return opts.GroupService.List(r.Context())
	})
	register("GET /workspace-groups/{id}", http.StatusOK, func(r *http.Request) (any, error) {
		return opts.GroupService.Get(r.Context(), r.PathValue("id"))
	})
	register("POST /workspace-groups", http.StatusCreated, func(r *http.Request) (any, error) {
		var input struct {
			Name string `json:"name"`
		}
		if err := decodeGroupInput(r, &input); err != nil {
			return nil, err
		}
		return opts.GroupService.Create(r.Context(), input.Name)
	})
	register("PUT /workspace-groups/{id}", http.StatusOK, func(r *http.Request) (any, error) {
		var input struct {
			Name string `json:"name"`
		}
		if err := decodeGroupInput(r, &input); err != nil {
			return nil, err
		}
		return opts.GroupService.Rename(r.Context(), r.PathValue("id"), input.Name)
	})
	register("DELETE /workspace-groups/{id}", http.StatusNoContent, func(r *http.Request) (any, error) {
		return nil, opts.GroupService.Delete(r.Context(), r.PathValue("id"))
	})
	register("POST /workspace-groups/{id}/members", http.StatusOK, func(r *http.Request) (any, error) {
		var input struct {
			WorkspaceIDs []string `json:"workspaceIds"`
		}
		if err := decodeGroupInput(r, &input); err != nil {
			return nil, err
		}
		return opts.GroupService.Add(r.Context(), r.PathValue("id"), input.WorkspaceIDs)
	})
	register("DELETE /workspace-groups/{id}/members/{workspaceId}", http.StatusOK, func(r *http.Request) (any, error) {
		return opts.GroupService.Remove(r.Context(), r.PathValue("id"), []string{r.PathValue("workspaceId")})
	})
	register("POST /workspace-groups/{id}/members/{workspaceId}/move", http.StatusOK, func(r *http.Request) (any, error) {
		var input struct {
			Before string `json:"before"`
		}
		if err := decodeGroupInput(r, &input); err != nil {
			return nil, err
		}
		return opts.GroupService.Move(r.Context(), r.PathValue("id"), r.PathValue("workspaceId"), input.Before)
	})
}

func writeGroupError(w http.ResponseWriter, err error) {
	code := http.StatusInternalServerError
	switch {
	case errors.Is(err, workspacegroups.ErrNotFound):
		code = http.StatusNotFound
	case errors.Is(err, workspacegroups.ErrInvalid):
		code = http.StatusBadRequest
	}
	writeError(w, code, cliErrors.ONE_CLI_ERROR, err.Error(), nil)
}

func decodeGroupInput(r *http.Request, value any) error {
	if err := decodeJSON(r, value); err != nil {
		return &groupInputError{error: i18n.Errorf("groups.invalid_input", err)}
	}
	return nil
}

type groupInputError struct{ error }

func (e *groupInputError) Unwrap() error { return workspacegroups.ErrInvalid }
