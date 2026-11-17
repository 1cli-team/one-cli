package serve

import (
	"errors"
	"net/http"
	"strings"

	workspaceapp "github.com/torchstellar-team/one-cli/packages/one-core/pkg/application/workspace"
	gateway "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
	gitmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/git"
	tracking "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/issuetracking"
	cliErrors "github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

type gatewayMappingInput struct {
	RepositoryID  string           `json:"repositoryId"`
	RepositoryIDs []string         `json:"repositoryIds,omitempty"`
	Base          string           `json:"base"`
	CheckTask     string           `json:"checkTask"`
	TeamID        string           `json:"teamId"`
	Flow          gateway.TeamFlow `json:"flow"`
}

func registerGatewayProjectRoutes(mux *http.ServeMux, opts MuxOpts) {
	mux.HandleFunc("GET /gateway/project-origins", handleGatewayProjectOrigins(opts))
	mux.HandleFunc("GET /gateway/project-bindings", func(w http.ResponseWriter, r *http.Request) {
		setNoStore(w)
		if opts.IssueTracking == nil {
			writeIssueError(w, tracking.Fail("unavailable"))
			return
		}
		rows, err := opts.IssueTracking.ListBindings(r.Context())
		if err != nil {
			writeIssueError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, rows)
	})
	register := func(pattern string, action func(http.ResponseWriter, *http.Request, *gateway.Client)) {
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			setNoStore(w)
			client, err := desktopGateway()
			if err != nil {
				writeError(w, http.StatusServiceUnavailable, cliErrors.ONE_CLI_ERROR, i18n.T("gateway.connection_invalid"), nil)
				return
			}
			if client == nil {
				writeError(w, http.StatusServiceUnavailable, cliErrors.ONE_CLI_ERROR, i18n.T("gateway.unavailable"), nil)
				return
			}
			action(w, r, client)
		})
	}
	reply := func(w http.ResponseWriter, result any, err error) {
		if err != nil {
			writeGatewayProjectError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
	register("GET /gateway/repositories", func(w http.ResponseWriter, r *http.Request, client *gateway.Client) {
		repositories, err := client.Repositories(r.Context())
		reply(w, repositories, err)
	})
	register("GET /gateway/repositories/{id}", func(w http.ResponseWriter, r *http.Request, client *gateway.Client) {
		repository, err := client.RepositoryChoices(r.Context(), r.PathValue("id"))
		reply(w, repository, err)
	})
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		register(method+" /gateway/project-mapping", func(w http.ResponseWriter, r *http.Request, client *gateway.Client) {
			if opts.IssueTracking == nil {
				writeIssueError(w, tracking.Fail("unavailable"))
				return
			}
			q := r.URL.Query()
			binding, err := opts.IssueTracking.Require(r.Context(), tracking.Scope{Kind: q.Get("kind"), ID: q.Get("scope")})
			if err != nil {
				writeIssueError(w, err)
				return
			}
			if r.Method == http.MethodGet {
				mapping, err := client.ProjectMapping(r.Context(), binding.Project.ID)
				reply(w, mapping, err)
				return
			}
			var input gatewayMappingInput
			if err := decodeJSON(r, &input); err != nil {
				writeError(w, http.StatusBadRequest, cliErrors.ONE_CLI_ERROR, i18n.T("gateway.invalid_mapping"), nil)
				return
			}
			if input.TeamID == "" {
				input.TeamID = binding.TeamID
			}
			mapping, err := client.SaveProjectMapping(r.Context(), binding.Project.ID, gateway.ProjectInput{
				OrganizationID: binding.OrganizationID, ConnectionID: binding.ConnectionID,
				RepositoryID: input.RepositoryID, RepositoryIDs: input.RepositoryIDs, Base: input.Base, CheckTask: input.CheckTask,
				TeamID: input.TeamID, Flow: input.Flow,
			})
			reply(w, mapping, err)
		})
	}
}

type gatewayProjectOrigin struct {
	Path       string                  `json:"path"`
	Name       string                  `json:"name,omitempty"`
	GitHub     string                  `json:"github"`
	HasOrigin  bool                    `json:"hasOrigin"`
	Repository gitmodule.RepositoryRef `json:"repository,omitzero"`
}

func handleGatewayProjectOrigins(opts MuxOpts) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setNoStore(w)
		if !requireGit(w, opts) {
			return
		}
		q := r.URL.Query()
		id := q.Get("scope")
		var workspaces []workspaceapp.RegisteredWorkspace
		switch q.Get("kind") {
		case "workspace":
			if opts.RegistryService == nil {
				writeIssueError(w, tracking.Fail("unavailable"))
				return
			}
			entry, err := opts.RegistryService.GitLocation(r.Context(), id)
			if err != nil {
				writeWorkspaceRegistryErr(w, id, err)
				return
			}
			workspaces = []workspaceapp.RegisteredWorkspace{entry}
		case "group":
			if opts.GroupService == nil {
				writeIssueError(w, tracking.Fail("unavailable"))
				return
			}
			group, err := opts.GroupService.Get(r.Context(), id)
			if err != nil {
				writeGroupError(w, err)
				return
			}
			workspaces = group.Workspaces
		default:
			writeIssueError(w, tracking.Fail("scope"))
			return
		}
		origins := make([]gatewayProjectOrigin, 0, len(workspaces))
		for _, entry := range workspaces {
			row := gatewayProjectOrigin{Path: entry.Root}
			if q.Get("kind") == "group" {
				row.Name = entry.Name
			}
			if entry.GitCommonDir == "" || entry.Status == workspaceapp.WorkspaceStatusMissing {
				origins = append(origins, row)
				continue
			}
			origin, err := opts.GitService.Origin(r.Context(), entry.Root)
			if err != nil {
				writeServiceError(w, err)
				return
			}
			row.HasOrigin = origin.FetchURL != ""
			row.Repository = gitmodule.ParseRepositoryRef(origin.FetchURL)
			if row.Repository.Host == gitmodule.GitHubHost {
				row.GitHub = strings.ToLower(row.Repository.Slug)
				row.Repository = gitmodule.RepositoryRef{}
			}
			origins = append(origins, row)
		}
		writeJSON(w, http.StatusOK, origins)
	}
}

func writeGatewayProjectError(w http.ResponseWriter, err error) {
	key := "gateway.project_request_failed"
	var response *gateway.ResponseError
	if errors.As(err, &response) {
		switch response.Reason {
		case "invalid_mapping", "repository_unavailable", "invalid_branch", "invalid_check",
			"linear_connection", "linear_project", "invalid_team", "invalid_states", "workflow_states", "config_write":
			key = "gateway." + response.Reason
		case "":
			if response.Code == http.StatusNotFound {
				key = "gateway.project_api_missing"
			}
		}
	}
	writeError(w, http.StatusBadGateway, cliErrors.ONE_CLI_ERROR, i18n.T(key), nil)
}
