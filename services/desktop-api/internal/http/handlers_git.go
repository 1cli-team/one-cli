package serve

import (
	"errors"
	"net/http"

	gitmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/git"
	cliErrors "github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

func registerGitRoutes(mux *http.ServeMux, opts MuxOpts) {
	mux.HandleFunc("GET /git/repositories", func(w http.ResponseWriter, r *http.Request) {
		if !requireGit(w, opts) {
			return
		}
		result, err := opts.GitService.Repositories(r.Context())
		if err != nil {
			writeRepositoryError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	})
	mux.HandleFunc("POST /git/repositories", handleAssociateRepository(opts))
	mux.HandleFunc("PUT /git/repositories/{repositoryId}", handleAssociateRepository(opts))
	mux.HandleFunc("GET /git/repositories/{repositoryId}", handleGetRepository(opts))
	mux.HandleFunc("POST /git/repositories/{repositoryId}/open", func(w http.ResponseWriter, r *http.Request) {
		if !requireGit(w, opts) {
			return
		}
		if err := opts.GitService.MarkRepositoryOpened(r.Context(), r.PathValue("repositoryId")); err != nil {
			writeRepositoryError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("DELETE /git/repositories/{repositoryId}", func(w http.ResponseWriter, r *http.Request) {
		if !requireGit(w, opts) {
			return
		}
		if err := opts.GitService.ForgetRepository(r.Context(), r.PathValue("repositoryId")); err != nil {
			writeRepositoryError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /git/clone", func(w http.ResponseWriter, r *http.Request) {
		if !requireGit(w, opts) {
			return
		}
		var input struct {
			URL  string `json:"url"`
			Path string `json:"path"`
		}
		if err := decodeJSON(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, cliErrors.ONE_CLI_ERROR, err.Error(), nil)
			return
		}
		result, err := opts.GitService.Clone(r.Context(), input.URL, input.Path)
		if err != nil {
			writeRepositoryError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, result)
	})

	mux.HandleFunc("PUT /workspaces/{entryId}/git/origin", handleGitWorkspace(opts, handleSetOrigin))
	mux.HandleFunc("POST /workspaces/{entryId}/git/branches", handleGitWorkspace(opts, handleCreateBranch))
	mux.HandleFunc("POST /git/repositories/{repositoryId}/branches", handleGitRepository(opts, handleCreateBranch))
	mux.HandleFunc("GET /workspaces/{entryId}/git/worktrees", handleGitWorkspace(opts, handleListWorktrees))
	mux.HandleFunc("POST /workspaces/{entryId}/git/worktrees", handleGitWorkspace(opts, handleAddWorktree))
	mux.HandleFunc("POST /workspaces/{entryId}/git/worktrees/{worktreeId}/open", handleGitWorkspace(opts, handleOpenWorktree))
	mux.HandleFunc("DELETE /workspaces/{entryId}/git/worktrees/{worktreeId}", handleGitWorkspace(opts, handleRemoveWorktree))
	mux.HandleFunc("GET /git/repositories/{repositoryId}/worktrees", handleGetRepository(opts))
	mux.HandleFunc("POST /git/repositories/{repositoryId}/worktrees", handleGitRepository(opts, handleAddWorktree))
	mux.HandleFunc("POST /git/repositories/{repositoryId}/worktrees/{worktreeId}/open", handleGitRepository(opts, handleOpenWorktree))
	mux.HandleFunc("DELETE /git/repositories/{repositoryId}/worktrees/{worktreeId}", handleGitRepository(opts, handleRemoveWorktree))
}

func handleAssociateRepository(opts MuxOpts) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireGit(w, opts) {
			return
		}
		var input struct {
			Path string `json:"path"`
		}
		if err := decodeJSON(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, cliErrors.ONE_CLI_ERROR, err.Error(), nil)
			return
		}
		var result *gitmodule.RegisteredRepository
		var err error
		if id := r.PathValue("repositoryId"); id != "" {
			result, err = opts.GitService.Relocate(r.Context(), id, input.Path)
		} else {
			result, err = opts.GitService.Associate(r.Context(), input.Path)
		}
		if err != nil {
			writeRepositoryError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func handleGitRepository(opts MuxOpts, factory workspaceHandlerFactory) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireGit(w, opts) {
			return
		}
		root, err := opts.GitService.ResolveRepository(r.Context(), r.PathValue("repositoryId"))
		if err != nil {
			writeRepositoryError(w, err)
			return
		}
		scoped := opts
		scoped.WorkspaceRoot = root
		factory(scoped).ServeHTTP(w, r)
	}
}

func handleListWorktrees(opts MuxOpts) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireGit(w, opts) {
			return
		}
		result, err := opts.GitService.List(r.Context(), opts.WorkspaceRoot)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func handleSetOrigin(opts MuxOpts) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setNoStore(w)
		var input gitmodule.SetOriginInput
		if err := decodeJSON(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, cliErrors.ONE_CLI_ERROR, i18n.T("git.origin_input_invalid"), nil)
			return
		}
		origin, err := opts.GitService.SetOrigin(r.Context(), opts.WorkspaceRoot, input)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, origin)
	}
}

func handleCreateBranch(opts MuxOpts) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireGit(w, opts) {
			return
		}
		var input gitmodule.CreateBranchInput
		if err := decodeJSON(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, cliErrors.ONE_CLI_ERROR, err.Error(), nil)
			return
		}
		result, err := opts.GitService.CreateBranch(r.Context(), opts.WorkspaceRoot, input)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, result)
	}
}

func handleAddWorktree(opts MuxOpts) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireGit(w, opts) {
			return
		}
		var input gitmodule.AddInput
		if err := decodeJSON(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, cliErrors.ONE_CLI_ERROR, err.Error(), nil)
			return
		}
		result, err := opts.GitService.Add(r.Context(), opts.WorkspaceRoot, input)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, result)
	}
}

func handleOpenWorktree(opts MuxOpts) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireGit(w, opts) {
			return
		}
		result, err := opts.GitService.Open(r.Context(), opts.WorkspaceRoot, r.PathValue("worktreeId"))
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func handleRemoveWorktree(opts MuxOpts) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireGit(w, opts) {
			return
		}
		if err := opts.GitService.Remove(r.Context(), opts.WorkspaceRoot, r.PathValue("worktreeId")); err != nil {
			writeServiceError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func writeRepositoryError(w http.ResponseWriter, err error) {
	if errors.Is(err, gitmodule.ErrRepositoryNotFound) {
		writeError(w, http.StatusNotFound, cliErrors.ONE_CLI_ERROR, err.Error(), nil)
		return
	}
	writeServiceError(w, err)
}

func requireGit(w http.ResponseWriter, opts MuxOpts) bool {
	if opts.GitService != nil {
		return true
	}
	writeError(w, http.StatusServiceUnavailable, cliErrors.ONE_CLI_ERROR, i18n.T("git.unavailable"), nil)
	return false
}

func handleGetRepository(opts MuxOpts) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireGit(w, opts) {
			return
		}
		result, err := opts.GitService.Repository(r.Context(), r.PathValue("repositoryId"))
		if err != nil {
			writeRepositoryError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func handleGitWorkspace(opts MuxOpts, factory workspaceHandlerFactory) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireGit(w, opts) {
			return
		}
		location, err := opts.RegistryService.GitLocation(r.Context(), r.PathValue("entryId"))
		if err != nil {
			writeWorkspaceRegistryErr(w, r.PathValue("entryId"), err)
			return
		}
		scoped := opts
		scoped.WorkspaceRoot = location.Root
		factory(scoped).ServeHTTP(w, r)
	}
}
