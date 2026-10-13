package serve

import (
	"errors"
	"net/http"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	manifestapp "github.com/torchstellar-team/one-cli/packages/cli/internal/application/manifest"
	workspaceapp "github.com/torchstellar-team/one-cli/packages/cli/internal/application/workspace"
	environmentmodule "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/environment"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

func handleBindWorkspaceEnvironment(opts MuxOpts) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setNoStore(w)
		if opts.WorkspaceRoot == "" {
			writeNoWorkspace(w)
			return
		}
		var input struct {
			Revision  string `json:"revision"`
			ProjectID string `json:"projectId"`
			Create    bool   `json:"create"`
		}
		if err := decodeJSON(r, &input); err != nil {
			writeBadPayload(w, err.Error())
			return
		}
		if strings.TrimSpace(input.Revision) == "" || (input.Create == (strings.TrimSpace(input.ProjectID) != "")) {
			writeBadPayload(w, i18n.T("env.bind.request_invalid"))
			return
		}
		binding, err := opts.EnvironmentService.BindWorkspace(r.Context(), environmentmodule.BindWorkspaceInput{
			Scope: execution.NewScope(r.Context(), opts.WorkspaceRoot), Revision: input.Revision,
			ProjectID: input.ProjectID, Create: input.Create,
		})
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, binding)
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
