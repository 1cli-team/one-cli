package serve

import (
	"net/http"
	"path/filepath"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/template"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	creationmodule "github.com/torchstellar-team/one-cli/packages/cli/internal/modules/creation"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

type projectTemplate struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Category    template.Category  `json:"category"`
	Directory   string             `json:"directory"`
	Toolchain   template.Toolchain `json:"toolchain"`
	Projects    []projectLocation  `json:"projects,omitempty"`
}
type projectLocation struct {
	Directory string `json:"directory"`
	Suffix    string `json:"suffix"`
}

func handleProjectTemplates() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		registry, err := template.Fetch(r.Context(), "")
		if err != nil {
			writeServiceError(w, err)
			return
		}
		templates := make([]projectTemplate, 0, len(registry.Templates))
		directories := map[template.Category]string{
			template.CategoryFrontend: "apps", template.CategoryBackend: "services", template.CategoryLibrary: "packages",
		}
		for _, entry := range registry.Templates {
			layouts, err := template.ProjectLayouts(strings.TrimPrefix(entry.Repo, template.LocalTemplatePrefix))
			if err != nil {
				writeServiceError(w, err)
				return
			}
			var locations []projectLocation
			for _, layout := range layouts {
				locations = append(locations, projectLocation{Directory: directories[layout.Category], Suffix: layout.Suffix})
			}
			templates = append(templates, projectTemplate{
				ID: entry.ID, Name: entry.DisplayName(), Description: entry.DisplayDescription(),
				Category: entry.Category, Directory: directories[entry.Category], Toolchain: entry.Toolchain,
				Projects: locations,
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"templates": templates})
	}
}

type createProjectRequest struct {
	Name       string `json:"name"`
	TemplateID string `json:"templateId"`
}

type createProjectResponse struct {
	Name        string                  `json:"name"`
	RelativeDir string                  `json:"relativeDir"`
	TemplateID  string                  `json:"templateId"`
	Warnings    []string                `json:"warnings,omitempty"`
	Projects    []createProjectResponse `json:"projects,omitempty"`
}

// The registry gateway resolves the selected Workspace; the request only
// supplies a project name and a built-in template ID, never filesystem paths.
func handleCreateProject(opts MuxOpts) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body createProjectRequest
		r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
		if err := decodeJSON(r, &body); err != nil {
			writeBadPayload(w, i18n.Tf("serve.project_payload_invalid", err))
			return
		}
		body.Name, body.TemplateID = strings.TrimSpace(body.Name), strings.TrimSpace(body.TemplateID)
		if !workspace.IsValidProjectName(body.Name) {
			writeServiceError(w, cliErrors.New(cliErrors.INVALID_NAME, i18n.Tf("add.name_invalid", body.Name)))
			return
		}
		if body.TemplateID == "" {
			writeServiceError(w, cliErrors.New(cliErrors.TEMPLATE_REQUIRED, i18n.T("add.template_required")))
			return
		}
		registry, err := template.Fetch(r.Context(), "")
		if err != nil {
			writeServiceError(w, err)
			return
		}
		var selected *template.Template
		for i := range registry.Templates {
			if registry.Templates[i].ID == body.TemplateID {
				selected = &registry.Templates[i]
				break
			}
		}
		if selected == nil {
			writeServiceError(w, cliErrors.New(cliErrors.TEMPLATE_NOT_FOUND, i18n.Tf("add.template_missing", body.TemplateID)))
			return
		}
		result, err := opts.CreationService.AddProject(r.Context(), opts.WorkspaceRoot, creationmodule.ProjectInput{
			Template: selected, Name: body.Name,
		})
		if err != nil {
			writeServiceError(w, err)
			return
		}
		relativeDir, err := filepath.Rel(opts.WorkspaceRoot, result.Project.TargetPath)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		response := createProjectResponse{
			Name: result.Project.Name, RelativeDir: filepath.ToSlash(relativeDir),
			TemplateID: result.Project.TemplateID, Warnings: result.Project.Warnings,
		}
		for _, project := range result.Project.Projects {
			rel, err := filepath.Rel(opts.WorkspaceRoot, project.TargetPath)
			if err != nil {
				writeServiceError(w, err)
				return
			}
			response.Projects = append(response.Projects, createProjectResponse{Name: project.Name, RelativeDir: filepath.ToSlash(rel), TemplateID: project.TemplateID})
		}
		writeJSON(w, http.StatusCreated, response)
	}
}
