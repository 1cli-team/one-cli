package serve

import (
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/template"
	"net/http"
	"strings"
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
	SkipSkills bool   `json:"skipSkills,omitempty"`
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
