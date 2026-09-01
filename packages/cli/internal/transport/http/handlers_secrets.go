package serve

import (
	"net/http"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/application/execution"
	environmentmodule "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/environment"
)

type createSecretRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type updateSecretRequest struct {
	Value string `json:"value"`
}

func registerSecretRoutes(mux *http.ServeMux, opts MuxOpts) {
	mux.HandleFunc("GET /workspace/secrets", handleListSecrets(opts))
	mux.HandleFunc("POST /workspace/secrets", handleCreateSecret(opts))
	mux.HandleFunc("GET /workspace/secrets/{key}", handleGetSecret(opts))
	mux.HandleFunc("PUT /workspace/secrets/{key}", handleUpdateSecret(opts))
	mux.HandleFunc("DELETE /workspace/secrets/{key}", handleDeleteSecret(opts))
}

func handleListSecrets(opts MuxOpts) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireSecretWorkspace(w, opts) {
			return
		}
		setNoStore(w)
		result, err := opts.EnvironmentService.List(r.Context(), environmentmodule.ListInput{
			Scope:       execution.NewScope(r.Context(), opts.WorkspaceRoot),
			Environment: r.URL.Query().Get("env"), Project: secretProject(r),
			RepositoryReadOnly: true,
		})
		if err != nil {
			writeProfileError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func handleGetSecret(opts MuxOpts) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireSecretWorkspace(w, opts) {
			return
		}
		setNoStore(w)
		result, err := opts.EnvironmentService.Get(r.Context(), environmentmodule.GetInput{
			Scope:       execution.NewScope(r.Context(), opts.WorkspaceRoot),
			Environment: r.URL.Query().Get("env"), Project: secretProject(r),
			Key: r.PathValue("key"), RepositoryReadOnly: true,
		})
		if err != nil {
			writeProfileError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func handleCreateSecret(opts MuxOpts) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireSecretWorkspace(w, opts) {
			return
		}
		setNoStore(w)
		var body createSecretRequest
		if err := decodeJSON(r, &body); err != nil {
			writeBadPayload(w, err.Error())
			return
		}
		if strings.TrimSpace(body.Key) == "" {
			writeBadPayload(w, "key is required")
			return
		}
		applySecretSet(w, r, opts, body.Key, body.Value, false)
	}
}

func handleUpdateSecret(opts MuxOpts) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireSecretWorkspace(w, opts) {
			return
		}
		setNoStore(w)
		var body updateSecretRequest
		if err := decodeJSON(r, &body); err != nil {
			writeBadPayload(w, err.Error())
			return
		}
		applySecretSet(w, r, opts, r.PathValue("key"), body.Value, true)
	}
}

func applySecretSet(
	w http.ResponseWriter,
	r *http.Request,
	opts MuxOpts,
	key, value string,
	overwrite bool,
) {
	project := secretProject(r)
	plan, err := opts.EnvironmentService.PlanSet(environmentmodule.PlanSetInput{
		Scope:       execution.NewScope(r.Context(), opts.WorkspaceRoot),
		Environment: r.URL.Query().Get("env"), Project: project,
	})
	if err != nil {
		writeProfileError(w, err)
		return
	}
	plan = plan.WithProject(project)
	result, err := opts.EnvironmentService.Set(r.Context(), environmentmodule.SetInput{
		Plan: plan, Key: key, Value: value, Overwrite: overwrite, RepositoryReadOnly: true,
	})
	if err != nil {
		writeProfileError(w, err)
		return
	}
	status := http.StatusOK
	if result.Action == "created" {
		status = http.StatusCreated
	}
	writeJSON(w, status, result)
}

func handleDeleteSecret(opts MuxOpts) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireSecretWorkspace(w, opts) {
			return
		}
		setNoStore(w)
		result, err := opts.EnvironmentService.Delete(r.Context(), environmentmodule.DeleteInput{
			Scope:       execution.NewScope(r.Context(), opts.WorkspaceRoot),
			Environment: r.URL.Query().Get("env"), Project: secretProject(r),
			Key: r.PathValue("key"), RepositoryReadOnly: true,
		})
		if err != nil {
			writeProfileError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func requireSecretWorkspace(w http.ResponseWriter, opts MuxOpts) bool {
	if opts.WorkspaceRoot == "" {
		writeNoWorkspace(w)
		return false
	}
	return true
}

func secretProject(r *http.Request) string { return strings.TrimSpace(r.URL.Query().Get("project")) }

func setNoStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
}
