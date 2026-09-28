package serve

import (
	"errors"
	"net/http"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	manifestapp "github.com/torchstellar-team/one-cli/packages/cli/internal/application/manifest"
	workspaceapp "github.com/torchstellar-team/one-cli/packages/cli/internal/application/workspace"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
)

func registerWorkspaceMutateRoutes(mux *http.ServeMux, opts MuxOpts) {
	// Profile bindings persist in machine-local One configuration. Manifest
	// publication has its own revision-checked, typed endpoint below.

	mux.HandleFunc(
		"POST /workspace/environment/backend/initialize",
		handleInitializeWorkspaceEnvironmentBackend(opts),
	)
	mux.HandleFunc("PUT /workspace/manifest", handlePutWorkspaceManifest(opts))
	mux.HandleFunc("POST /workspace/manifest/preview", handlePreviewWorkspaceManifest(opts))

	// Keep the former repository-mutation paths stable for older Dashboard
	// clients, but reject them before reading a body or resolving a workspace.
	for _, pattern := range []string{
		"PUT /workspace/projects/{name}",
		"PUT /workspace/projects/{name}/environment",
	} {
		mux.HandleFunc(pattern, handleRepositoryReadOnly())
	}
}

func handleInitializeWorkspaceEnvironmentBackend(opts MuxOpts) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if opts.WorkspaceRoot == "" {
			writeNoWorkspace(w)
			return
		}
		if err := opts.EnvironmentService.EnsureInfisicalReady(
			r.Context(),
			execution.NewScope(r.Context(), opts.WorkspaceRoot),
			r.URL.Query().Get("env"),
			secretProject(r),
		); err != nil {
			writeServiceError(w, err)
			return
		}
		settings, err := opts.WorkspaceService.WorkspaceEnvironment(
			r.Context(), opts.WorkspaceRoot, r.URL.Query().Get("env"),
		)
		if err != nil {
			writeWorkspaceMutationErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, settings)
	}
}

func handlePutWorkspaceManifest(opts MuxOpts) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if opts.WorkspaceRoot == "" {
			writeNoWorkspace(w)
			return
		}
		var body manifestapp.ApplyManifestInput
		if err := decodeJSON(r, &body); err != nil {
			writeBadPayload(w, err.Error())
			return
		}
		result, err := opts.ManifestService.ApplyManifestDraft(r.Context(), opts.WorkspaceRoot, body)
		if err != nil {
			writeWorkspaceMutationErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func handlePreviewWorkspaceManifest(opts MuxOpts) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if opts.WorkspaceRoot == "" {
			writeNoWorkspace(w)
			return
		}
		var body manifestapp.PreviewManifestInput
		if err := decodeJSON(r, &body); err != nil {
			writeBadPayload(w, err.Error())
			return
		}
		result, err := opts.ManifestService.PreviewManifestDraft(r.Context(), opts.WorkspaceRoot, body)
		if err != nil {
			writeWorkspaceMutationErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func handleRepositoryReadOnly() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeError(
			w,
			http.StatusConflict,
			cliErrors.SERVE_REPOSITORY_READ_ONLY,
			"This legacy route cannot write repository configuration; use the revision-checked /manifest endpoint.",
			nil,
		)
	}
}

func writeWorkspaceMutationErr(w http.ResponseWriter, err error) {
	var conflict *manifestapp.ManifestConflict
	switch {
	case errors.As(err, &conflict):
		writeError(w, http.StatusConflict, cliErrors.SERVE_MANIFEST_CONFLICT, err.Error(), map[string]any{
			"expected_revision": conflict.Expected,
			"current_revision":  conflict.Current,
		})
	case errors.Is(err, manifestapp.ErrInvalidInput), errors.Is(err, workspaceapp.ErrInvalidInput):
		writeBadPayload(w, err.Error())
	case errors.Is(err, manifestapp.ErrProjectNotFound), errors.Is(err, workspaceapp.ErrProjectNotFound):
		writeNotFound(w, err.Error())
	default:
		writeManifestErr(w, err)
	}
}

func writeNoWorkspace(w http.ResponseWriter) {
	writeError(w, http.StatusConflict, cliErrors.NOT_ONE_PROJECT,
		"`one serve` was launched outside a One workspace — there is no workspace to inspect.",
		nil)
}

func writeBadPayload(w http.ResponseWriter, message string) {
	writeError(w, http.StatusBadRequest, cliErrors.SERVE_PAYLOAD_INVALID, message, nil)
}

func writeNotFound(w http.ResponseWriter, message string) {
	writeError(w, http.StatusNotFound, cliErrors.ONE_CLI_ERROR, message, nil)
}

func writeManifestErr(w http.ResponseWriter, err error) {
	message := err.Error()
	writeError(w, http.StatusInternalServerError, cliErrors.MANIFEST_INVALID, message, nil)
}
