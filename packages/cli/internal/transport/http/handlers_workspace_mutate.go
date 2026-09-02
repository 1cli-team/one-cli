package serve

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	manifestapp "github.com/torchstellar-team/one-cli/packages/cli/internal/application/manifest"
	workspaceapp "github.com/torchstellar-team/one-cli/packages/cli/internal/application/workspace"
	workspacecore "github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	environmentmodule "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/environment"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
)

func registerWorkspaceMutateRoutes(mux *http.ServeMux, opts MuxOpts) {
	// Profile bindings persist in machine-local One configuration. Manifest
	// publication has its own revision-checked, typed endpoint below.
	mux.HandleFunc("PUT /workspace/profile-bindings/env", handlePutWorkspaceEnvironmentProfile(opts))
	mux.HandleFunc("PUT /workspace/environment/backend", handlePutWorkspaceEnvironmentBackend(opts))
	mux.HandleFunc(
		"POST /workspace/environment/backend/initialize",
		handleInitializeWorkspaceEnvironmentBackend(opts),
	)
	mux.HandleFunc(
		"PUT /workspace/projects/{name}/profile-bindings/{domain}",
		handlePutProjectProfileBinding(opts),
	)
	mux.HandleFunc("PUT /workspace/manifest", handlePutWorkspaceManifest(opts))
	mux.HandleFunc("POST /workspace/manifest/preview", handlePreviewWorkspaceManifest(opts))

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
			writeProfileError(w, err)
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

type workspaceEnvironmentBackendReq struct {
	Revision string `json:"revision"`
	Backend  string `json:"backend"`
}

// handlePutWorkspaceEnvironmentBackend publishes a reviewed backend switch
// through the same workflow as `one env switch`. In particular, selecting
// Infisical initializes and persists its project binding before returning.
func handlePutWorkspaceEnvironmentBackend(opts MuxOpts) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if opts.WorkspaceRoot == "" {
			writeNoWorkspace(w)
			return
		}
		var body workspaceEnvironmentBackendReq
		if err := decodeJSON(r, &body); err != nil {
			writeBadPayload(w, err.Error())
			return
		}
		body.Revision = strings.TrimSpace(body.Revision)
		body.Backend = strings.TrimSpace(body.Backend)
		if body.Revision == "" || body.Backend == "" {
			writeBadPayload(w, "revision and backend are required")
			return
		}
		_, currentRevision, err := workspacecore.ReadManifestSnapshot(opts.WorkspaceRoot)
		if err != nil {
			writeWorkspaceMutationErr(w, err)
			return
		}
		if body.Revision != currentRevision {
			writeWorkspaceMutationErr(w, &manifestapp.ManifestConflict{
				Expected: body.Revision,
				Current:  currentRevision,
			})
			return
		}

		scope := execution.NewScope(r.Context(), opts.WorkspaceRoot)
		plan, err := opts.EnvironmentService.PlanSwitch(scope, body.Backend)
		if err != nil {
			writeProfileError(w, err)
			return
		}
		if _, err := opts.EnvironmentService.Switch(r.Context(), plan, environmentmodule.SwitchOptions{
			Environment: r.URL.Query().Get("env"),
		}); err != nil {
			writeProfileError(w, err)
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
	if errors.Is(err, workspacecore.ErrEnvBackendNotConfigured) {
		writeError(w, http.StatusConflict, cliErrors.ONE_CLI_ERROR, message, nil)
		return
	}
	writeError(w, http.StatusInternalServerError, cliErrors.MANIFEST_INVALID, message, nil)
}
