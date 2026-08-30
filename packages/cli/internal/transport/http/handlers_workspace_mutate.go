package serve

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	workspaceapp "github.com/torchstellar-team/one-cli/packages/cli/internal/application/workspace"
	workspacecore "github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
)

func registerWorkspaceMutateRoutes(mux *http.ServeMux, opts MuxOpts) {
	// Profile bindings are the Dashboard's only writable Workspace surface.
	// They persist in machine-local One configuration, never in the repository.
	mux.HandleFunc("PUT /workspace/profile-bindings/env", handlePutWorkspaceEnvironmentProfile(opts))
	mux.HandleFunc(
		"PUT /workspace/projects/{name}/profile-bindings/{domain}",
		handlePutProjectProfileBinding(opts),
	)

	// Keep the former repository-mutation paths stable for older Dashboard
	// clients, but reject them before reading a body or resolving a workspace.
	for _, pattern := range []string{
		"PUT /workspace/projects/{name}",
		"PUT /workspace/projects/{name}/environment",
		"PUT /workspace/projects/{name}/deploy",
		"PUT /workspace/projects/{name}/container",
		"PUT /workspace/projects/{name}/settings/deploy",
		"PUT /workspace/projects/{name}/settings/container",
	} {
		mux.HandleFunc(pattern, handleRepositoryReadOnly())
	}
}

type workspaceProfileBindingReq struct {
	Profile *string `json:"profile"`
}

func handlePutWorkspaceEnvironmentProfile(opts MuxOpts) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if opts.WorkspaceRoot == "" {
			writeNoWorkspace(w)
			return
		}
		body, ok := decodeProfileBinding(w, r)
		if !ok {
			return
		}
		settings, err := opts.WorkspaceService.UpdateWorkspaceEnvironmentProfile(
			r.Context(), opts.WorkspaceRoot, r.URL.Query().Get("env"), *body.Profile,
		)
		if err != nil {
			writeWorkspaceMutationErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, settings)
	}
}

func handlePutProjectProfileBinding(opts MuxOpts) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if opts.WorkspaceRoot == "" {
			writeNoWorkspace(w)
			return
		}
		body, ok := decodeProfileBinding(w, r)
		if !ok {
			return
		}
		settings, err := opts.WorkspaceService.UpdateProjectProfileBinding(
			r.Context(),
			opts.WorkspaceRoot,
			r.PathValue("name"),
			r.PathValue("domain"),
			r.URL.Query().Get("env"),
			*body.Profile,
		)
		if err != nil {
			writeWorkspaceMutationErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, settings)
	}
}

func decodeProfileBinding(
	w http.ResponseWriter,
	r *http.Request,
) (workspaceProfileBindingReq, bool) {
	var body workspaceProfileBindingReq
	if r.Body == nil {
		writeBadPayload(w, "empty body")
		return workspaceProfileBindingReq{}, false
	}
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		writeBadPayload(w, err.Error())
		return workspaceProfileBindingReq{}, false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("request body must contain exactly one JSON object")
		}
		writeBadPayload(w, err.Error())
		return workspaceProfileBindingReq{}, false
	}
	if body.Profile == nil {
		writeBadPayload(w, "profile is required")
		return workspaceProfileBindingReq{}, false
	}
	return body, true
}

func handleRepositoryReadOnly() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeError(
			w,
			http.StatusConflict,
			cliErrors.SERVE_REPOSITORY_READ_ONLY,
			"Dashboard treats the workspace repository as read-only; edit one.manifest.json in code review instead.",
			nil,
		)
	}
}

func writeWorkspaceMutationErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, workspaceapp.ErrInvalidInput):
		writeBadPayload(w, err.Error())
	case errors.Is(err, workspaceapp.ErrProjectNotFound):
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
	if errors.Is(err, workspacecore.ErrEnvBackendNotConfigured) {
		writeError(w, http.StatusConflict, cliErrors.ONE_CLI_ERROR, message, nil)
		return
	}
	writeError(w, http.StatusInternalServerError, cliErrors.MANIFEST_INVALID, message, nil)
}
