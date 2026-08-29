package deployment

import (
	"path/filepath"
	"strings"

	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/template"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
)

// Target is one project deployment job resolved from the workspace manifest.
type Target struct {
	Project        workspace.Project
	Backend        string
	Toolchain      string
	TemplateID     string
	PackageManager string
}

func configuredTargets(projectRoot string, manifest *workspace.Manifest) ([]Target, error) {
	if !workspace.HasManifest(projectRoot) {
		return nil, cliErrors.New(cliErrors.NOT_ONE_PROJECT,
			"未检测到 One CLI 项目，请在工作区根目录执行。")
	}
	if manifest == nil {
		return nil, cliErrors.New(cliErrors.ONE_CLI_ERROR, "deploy manifest 不能为空")
	}
	out := make([]Target, 0, len(manifest.Projects))
	for index := range manifest.Projects {
		project := &manifest.Projects[index]
		selection := workspace.DeployForProject(manifest, project.Name)
		if selection.Backend == "" {
			continue
		}
		out = append(out, manifestProjectTarget(projectRoot, project, selection.Backend))
	}
	return out, nil
}

func manifestProjectTarget(root string, project *workspace.ManifestProject, backend string) Target {
	if project == nil {
		return Target{}
	}
	return Target{
		Project: workspace.Project{
			Name: project.Name, RelativeDir: project.RelativeDir,
			TargetDir: filepath.Join(root, filepath.FromSlash(project.RelativeDir)),
			Toolchain: project.Toolchain, PackageManager: project.PackageManager, TemplateID: project.TemplateID,
		},
		Backend: backend, Toolchain: project.Toolchain,
		TemplateID: project.TemplateID, PackageManager: project.PackageManager,
	}
}

func findManifestProject(manifest *workspace.Manifest, selector string) *workspace.ManifestProject {
	if manifest == nil {
		return nil
	}
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return nil
	}
	for index := range manifest.Projects {
		if manifest.Projects[index].Name == selector {
			return &manifest.Projects[index]
		}
	}
	pathSelector := strings.TrimSuffix(strings.TrimPrefix(selector, "./"), "/")
	pathSelector = workspace.ToPosixPath(pathSelector)
	for index := range manifest.Projects {
		if manifest.Projects[index].RelativeDir == pathSelector {
			return &manifest.Projects[index]
		}
	}
	return nil
}

func templateForProject(registry *template.Registry, project *workspace.ManifestProject) *template.Template {
	if registry == nil || project == nil {
		return nil
	}
	for index := range registry.Templates {
		if registry.Templates[index].ID == project.TemplateID {
			return &registry.Templates[index]
		}
	}
	return nil
}

func compatibleBackends(backendCatalog *catalog.Catalog, projectTemplate *template.Template) []string {
	if backendCatalog == nil || projectTemplate == nil || projectTemplate.Compat == nil {
		return nil
	}
	registered := map[string]bool{}
	for _, backend := range backendCatalog.ForDomain(catalog.DomainDeploy) {
		if backend.Has(catalog.CapabilityDeploy) {
			registered[backend.ID.Name] = true
		}
	}
	result := make([]string, 0, len(projectTemplate.Compat["deploy"]))
	for _, id := range projectTemplate.Compat["deploy"] {
		if registered[id] {
			result = append(result, id)
		}
	}
	return result
}
