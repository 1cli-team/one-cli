package serve

// handlers_workspace.go exposes read-only views of the workspace
// `one serve` was launched in:
//
//	GET /workspace/overview  → Overview envelope (one-cli/workspace-overview/v1)
//	GET /workspace/projects/{name} → Project settings (one-cli/workspace-project/v1)
//
// The dashboard's home page reads this; if Present is false (no workspace
// at launch time) it falls back to the profile-editor landing.

import (
	"net/http"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
)

func registerWorkspaceRoutes(mux *http.ServeMux, opts MuxOpts) {
	mux.HandleFunc("GET /workspace/overview", handleGetWorkspaceOverview(opts))
	mux.HandleFunc("GET /workspace/profile-bindings/env", handleGetWorkspaceEnvironmentProfile(opts))
	mux.HandleFunc("GET /workspace/projects/{name}", handleGetWorkspaceProject(opts))
}

func handleGetWorkspaceOverview(opts MuxOpts) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ov, err := opts.WorkspaceService.Overview(opts.WorkspaceRoot, r.URL.Query().Get("env"))
		if err != nil {
			writeError(w, http.StatusInternalServerError, cliErrors.MANIFEST_INVALID,
				err.Error(), map[string]any{"root": opts.WorkspaceRoot})
			return
		}
		writeJSON(w, http.StatusOK, ov)
	}
}

func handleGetWorkspaceProject(opts MuxOpts) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if opts.WorkspaceRoot == "" {
			writeNoWorkspace(w)
			return
		}
		settings, err := opts.WorkspaceService.ProjectSettings(
			r.Context(), opts.WorkspaceRoot, r.PathValue("name"), r.URL.Query().Get("env"),
		)
		if err != nil {
			writeWorkspaceMutationErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, settings)
	}
}

func handleGetWorkspaceEnvironmentProfile(opts MuxOpts) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if opts.WorkspaceRoot == "" {
			writeNoWorkspace(w)
			return
		}
		settings, err := opts.WorkspaceService.WorkspaceEnvironmentProfile(
			r.Context(), opts.WorkspaceRoot, r.URL.Query().Get("env"),
		)
		if err != nil {
			writeWorkspaceMutationErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, settings)
	}
}
