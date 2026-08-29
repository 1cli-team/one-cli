package serve

import (
	"errors"
	"net/http"

	workspaceapp "github.com/torchstellar-team/one-cli/packages/cli/internal/application/workspace"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
)

func registerWorkspaceMutateRoutes(mux *http.ServeMux, opts MuxOpts) {
	mux.HandleFunc("PUT /workspace/domains/env", handlePutWorkspaceEnv(opts))
	mux.HandleFunc("PUT /workspace/projects/{name}/deploy", handlePutProjectDeploy(opts))
	mux.HandleFunc("PUT /workspace/projects/{name}/container", handlePutProjectContainer(opts))
}

type envInitReq struct {
	Kind string `json:"kind"`
}

func handlePutWorkspaceEnv(opts MuxOpts) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if opts.WorkspaceRoot == "" {
			writeNoWorkspace(w)
			return
		}
		var body envInitReq
		if err := decodeJSON(r, &body); err != nil {
			writeBadPayload(w, err.Error())
			return
		}
		overview, err := opts.WorkspaceService.SetEnvironment(opts.WorkspaceRoot, body.Kind)
		if err != nil {
			writeWorkspaceMutationErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, overview)
	}
}

type deployInitReq struct {
	Kind string `json:"kind"`
}

func handlePutProjectDeploy(opts MuxOpts) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if opts.WorkspaceRoot == "" {
			writeNoWorkspace(w)
			return
		}
		var body deployInitReq
		if err := decodeJSON(r, &body); err != nil {
			writeBadPayload(w, err.Error())
			return
		}
		overview, err := opts.WorkspaceService.SetProjectDeployment(
			r.Context(), opts.WorkspaceRoot, r.PathValue("name"), body.Kind,
		)
		if err != nil {
			writeWorkspaceMutationErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, overview)
	}
}

type containerInitReq struct {
	Kind  string `json:"kind,omitempty"`
	Image string `json:"image,omitempty"`
}

func handlePutProjectContainer(opts MuxOpts) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if opts.WorkspaceRoot == "" {
			writeNoWorkspace(w)
			return
		}
		var body containerInitReq
		if err := decodeJSON(r, &body); err != nil {
			writeBadPayload(w, err.Error())
			return
		}
		overview, err := opts.WorkspaceService.SetProjectContainer(
			opts.WorkspaceRoot, r.PathValue("name"), body.Kind, body.Image,
		)
		if err != nil {
			writeWorkspaceMutationErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, overview)
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
		"`one serve` was launched outside a One workspace — there is no manifest to mutate.",
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
	if errors.Is(err, workspace.ErrEnvBackendNotConfigured) {
		writeError(w, http.StatusConflict, cliErrors.ONE_CLI_ERROR, message, nil)
		return
	}
	writeError(w, http.StatusInternalServerError, cliErrors.MANIFEST_INVALID, message, nil)
}
