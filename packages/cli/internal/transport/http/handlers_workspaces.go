package serve

// handlers_workspaces.go exposes the machine-local Workspace registry. The
// singular /workspace/* routes remain pinned to the directory `one serve`
// launched from; these plural routes first resolve an opaque registry entry
// ID and then reuse the same transport handlers against that server-validated
// root. A request can therefore never select an arbitrary filesystem path.

import (
	"errors"
	"net/http"

	workspaceapp "github.com/torchstellar-team/one-cli/packages/cli/internal/application/workspace"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
)

func registerWorkspacesRoutes(mux *http.ServeMux, opts MuxOpts) {
	mux.HandleFunc("GET /workspaces", handleListWorkspaces(opts))
	mux.HandleFunc("DELETE /workspaces/{entryId}", handleForgetWorkspace(opts))

	mux.HandleFunc("GET /workspaces/{entryId}/overview",
		handleResolvedWorkspaceRead(opts, handleGetWorkspaceOverview))
	mux.HandleFunc("GET /workspaces/{entryId}/profile-bindings/env",
		handleResolvedWorkspaceRead(opts, handleGetWorkspaceEnvironmentProfile))
	mux.HandleFunc("GET /workspaces/{entryId}/projects/{name}",
		handleResolvedWorkspaceRead(opts, handleGetWorkspaceProject))

	mux.HandleFunc("PUT /workspaces/{entryId}/profile-bindings/env",
		handleResolvedWorkspace(opts, handlePutWorkspaceEnvironmentProfile))
	mux.HandleFunc("PUT /workspaces/{entryId}/environment/backend",
		handleResolvedWorkspace(opts, handlePutWorkspaceEnvironmentBackend))
	mux.HandleFunc("POST /workspaces/{entryId}/environment/backend/initialize",
		handleResolvedWorkspace(opts, handleInitializeWorkspaceEnvironmentBackend))
	mux.HandleFunc("PUT /workspaces/{entryId}/projects/{name}/profile-bindings/{domain}",
		handleResolvedWorkspace(opts, handlePutProjectProfileBinding))
	mux.HandleFunc("PUT /workspaces/{entryId}/manifest",
		handleResolvedWorkspace(opts, handlePutWorkspaceManifest))
	mux.HandleFunc("POST /workspaces/{entryId}/manifest/preview",
		handleResolvedWorkspace(opts, handlePreviewWorkspaceManifest))
	mux.HandleFunc("GET /workspaces/{entryId}/secrets",
		handleResolvedWorkspace(opts, handleListSecrets))
	mux.HandleFunc("POST /workspaces/{entryId}/secrets",
		handleResolvedWorkspace(opts, handleCreateSecret))
	mux.HandleFunc("GET /workspaces/{entryId}/secrets/{key}",
		handleResolvedWorkspace(opts, handleGetSecret))
	mux.HandleFunc("PUT /workspaces/{entryId}/secrets/{key}",
		handleResolvedWorkspace(opts, handleUpdateSecret))
	mux.HandleFunc("DELETE /workspaces/{entryId}/secrets/{key}",
		handleResolvedWorkspace(opts, handleDeleteSecret))

	for _, pattern := range []string{
		"PUT /workspaces/{entryId}/projects/{name}",
		"PUT /workspaces/{entryId}/projects/{name}/environment",
		"PUT /workspaces/{entryId}/projects/{name}/deploy",
		"PUT /workspaces/{entryId}/projects/{name}/container",
		"PUT /workspaces/{entryId}/projects/{name}/settings/deploy",
		"PUT /workspaces/{entryId}/projects/{name}/settings/container",
	} {
		mux.HandleFunc(pattern, handleRepositoryReadOnly())
	}
}

func handleListWorkspaces(opts MuxOpts) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if opts.RegistryService == nil {
			// Backward-compatible composition: older embedders do not have a
			// registry. Keep the endpoint stable and explicit instead of
			// constructing a repository with ambient machine state here.
			writeJSON(w, http.StatusOK, map[string]any{
				"schema":     workspaceapp.WorkspaceRegistrySchema,
				"workspaces": []any{},
			})
			return
		}
		response, err := opts.RegistryService.List(r.Context(), opts.WorkspaceRoot)
		if err != nil {
			writeError(w, http.StatusInternalServerError, cliErrors.ONE_CLI_ERROR,
				err.Error(), nil)
			return
		}
		writeJSON(w, http.StatusOK, response)
	}
}

func handleForgetWorkspace(opts MuxOpts) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entryID := r.PathValue("entryId")
		if opts.RegistryService == nil {
			writeWorkspaceRegistryErr(w, entryID, workspaceapp.ErrRegistryEntryNotFound)
			return
		}
		if err := opts.RegistryService.Forget(r.Context(), entryID); err != nil {
			writeWorkspaceRegistryErr(w, entryID, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

type workspaceHandlerFactory func(MuxOpts) http.HandlerFunc

func handleResolvedWorkspaceRead(opts MuxOpts, factory workspaceHandlerFactory) http.HandlerFunc {
	return handleResolvedWorkspaceWithMode(opts, factory, true)
}

// handleResolvedWorkspace is the only gateway from an entryId to a root. It
// deliberately resolves on every request: RegistryService re-reads the
// manifest before a handler gets the selected root. Read-only projections may
// inspect an identity conflict; mutations fail closed until it is repaired.
func handleResolvedWorkspace(opts MuxOpts, factory workspaceHandlerFactory) http.HandlerFunc {
	return handleResolvedWorkspaceWithMode(opts, factory, false)
}

func handleResolvedWorkspaceWithMode(
	opts MuxOpts,
	factory workspaceHandlerFactory,
	readOnly bool,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entryID := r.PathValue("entryId")
		if opts.RegistryService == nil {
			writeWorkspaceRegistryErr(w, entryID, workspaceapp.ErrRegistryEntryNotFound)
			return
		}
		var resolved workspaceapp.ResolvedWorkspace
		var err error
		if readOnly {
			resolved, err = opts.RegistryService.ResolveRead(r.Context(), entryID)
		} else {
			resolved, err = opts.RegistryService.Resolve(r.Context(), entryID)
		}
		if err != nil {
			writeWorkspaceRegistryErr(w, entryID, err)
			return
		}
		scoped := opts
		scoped.WorkspaceRoot = resolved.Root
		factory(scoped).ServeHTTP(w, r)
	}
}

func writeWorkspaceRegistryErr(w http.ResponseWriter, entryID string, err error) {
	context := map[string]any{"entryId": entryID}
	switch {
	case errors.Is(err, workspaceapp.ErrRegistryEntryNotFound):
		writeError(w, http.StatusNotFound, cliErrors.ONE_CLI_ERROR,
			"workspace registry entry was not found.", context)
	case errors.Is(err, workspaceapp.ErrRegistryUnavailable),
		errors.Is(err, workspaceapp.ErrRegistryIdentityConflict):
		writeError(w, http.StatusConflict, cliErrors.ONE_CLI_ERROR, err.Error(), context)
	default:
		writeError(w, http.StatusInternalServerError, cliErrors.ONE_CLI_ERROR, err.Error(), context)
	}
}
