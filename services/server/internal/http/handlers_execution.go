package serve

import (
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/application/execution"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/ports/secrets"
	env "github.com/torchstellar-team/one-cli/services/desktop-api/internal/application/environment"
	provider "github.com/torchstellar-team/one-cli/services/desktop-api/internal/providers/infisical"
	"net/http"
)

// These operations resolve configuration for execution; no command, file path,
// executable or subprocess is ever accepted by this API.
func registerExecutionRoutes(api *http.ServeMux, opts MuxOpts) {
	api.HandleFunc("POST /workspaces/{entryId}/operations/{operation}", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Environment string   `json:"environment"`
			Project     string   `json:"project"`
			Key         string   `json:"key"`
			Value       string   `json:"value"`
			Overwrite   bool     `json:"overwrite"`
			Create      bool     `json:"create"`
			ProjectID   string   `json:"projectId"`
			Revision    string   `json:"revision"`
			Dirs        []string `json:"dirs"`
		}
		if err := decodeJSON(r, &body); err != nil {
			writeBadPayload(w, err.Error())
			return
		}
		id := r.PathValue("entryId")
		scope := execution.NewScope(r.Context(), id).Derive(execution.ScopePatch{WorkspaceRoot: id, Project: body.Project})
		var result any
		var err error
		switch r.PathValue("operation") {
		case "summary":
			result, err = opts.EnvironmentService.Summary(scope)
		case "get":
			result, err = opts.EnvironmentService.Get(r.Context(), env.GetInput{Scope: scope, Environment: body.Environment, Project: body.Project, Key: body.Key})
		case "list":
			result, err = opts.EnvironmentService.List(r.Context(), env.ListInput{Scope: scope, Environment: body.Environment, Project: body.Project})
		case "plan-set":
			result, err = opts.EnvironmentService.PlanSet(env.PlanSetInput{Scope: scope, Environment: body.Environment, Project: body.Project})
		case "set":
			plan, e := opts.EnvironmentService.PlanSet(env.PlanSetInput{Scope: scope, Environment: body.Environment, Project: body.Project})
			if e != nil {
				err = e
				break
			}
			result, err = opts.EnvironmentService.Set(r.Context(), env.SetInput{Plan: plan.WithProject(body.Project), Key: body.Key, Value: body.Value, Overwrite: body.Overwrite})
		case "delete":
			result, err = opts.EnvironmentService.Delete(r.Context(), env.DeleteInput{Scope: scope, Environment: body.Environment, Project: body.Project, Key: body.Key})
		case "bind":
			result, err = opts.EnvironmentService.BindWorkspace(r.Context(), env.BindWorkspaceInput{Scope: scope, ProjectID: body.ProjectID, Create: body.Create, Revision: body.Revision})
		case "snapshot":
			m, _, e := opts.Store.Load(r.Context(), id)
			if e != nil {
				err = e
				break
			}
			if m.Env == nil {
				out := map[string]map[string]string{}
				for _, dir := range body.Dirs {
					out[dir] = map[string]string{}
				}
				result = out
				break
			}
			allowed := map[string]bool{"": true}
			for _, p := range m.Projects {
				allowed[p.RelativeDir] = true
			}
			for _, dir := range body.Dirs {
				if !allowed[dir] {
					writeBadPayload(w, i18n.ForContext(r.Context()).T("server.project_directory_invalid"))
					return
				}
			}
			_, _, e = secrets.ResolveEnvNameContext(r.Context(), m, body.Environment, false)
			if e != nil {
				err = e
				break
			}
			result, err = provider.FetchConfigured(r.Context(), &provider.WorkspaceConfig{SiteURL: m.Env.SiteURL, ProjectID: m.Env.ProjectID, Environments: m.Env.Environments}, body.Dirs, body.Environment)
		default:
			http.NotFound(w, r)
			return
		}
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, 200, result)
	})
	api.HandleFunc("GET /global-env/location", func(w http.ResponseWriter, r *http.Request) {
		location, environments, err := env.GlobalSummary(r.Context())
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"location": location, "environments": environments})
	})
	api.HandleFunc("PUT /global-env/location", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ProjectID   string `json:"projectId"`
			Environment string `json:"environment"`
		}
		if err := decodeJSON(r, &body); err != nil {
			writeBadPayload(w, err.Error())
			return
		}
		result, err := env.BindGlobal(r.Context(), body.ProjectID, body.Environment)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, 200, result)
	})
	api.HandleFunc("POST /global-env/values", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Environment string   `json:"environment"`
			Path        string   `json:"path"`
			Keys        []string `json:"keys"`
		}
		if err := decodeJSON(r, &body); err != nil {
			writeBadPayload(w, err.Error())
			return
		}
		values, err := env.GlobalValues(r.Context(), body.Environment, body.Path, body.Keys)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, 200, values)
	})
}
