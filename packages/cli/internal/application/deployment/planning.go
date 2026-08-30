package deployment

import (
	"strings"

	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/template"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
)

// PlanRequest contains the transport-neutral choices that affect which
// projects a deploy command should execute or configure.
type PlanRequest struct {
	ProjectRoot string
	Manifest    *workspace.Manifest
	Templates   *template.Registry
	Project     string
	Backend     string
}

// TargetPlan is one of three states:
//   - Targets is non-empty when deployment can execute immediately;
//   - ProjectChoices asks the transport to choose a deployable project;
//   - Setup asks the transport to collect any interactive profile/backend
//     input before configuring a project's first deployment.
type TargetPlan struct {
	Targets        []Target
	ProjectChoices []string
	Setup          *TargetSetup
}

// TargetSetup describes a compatible first-deployment configuration without
// owning prompts or workspace mutation. Backend is empty when the transport
// still needs to choose one of CompatibleBackends.
type TargetSetup struct {
	Project            *workspace.ManifestProject
	Template           *template.Template
	CompatibleBackends []string
	Backend            string
}

// PlanTargets owns the reusable target-selection policy while leaving all
// interactive choices to the transport.
func (s *Service) PlanTargets(request PlanRequest) (TargetPlan, error) {
	configured, err := configuredTargets(request.ProjectRoot, request.Manifest)
	if err != nil {
		return TargetPlan{}, err
	}

	selector := strings.TrimSpace(request.Project)
	backend := normalizeBackend(request.Backend)
	if selector == "" && backend == "" && len(configured) > 0 {
		return TargetPlan{Targets: configured}, nil
	}

	project := findManifestProject(request.Manifest, selector)
	if selector == "" {
		if request.Manifest != nil && len(request.Manifest.Projects) == 1 && backend != "" {
			project = &request.Manifest.Projects[0]
		} else {
			choices := deployableProjectNames(s.catalog, request.Manifest, request.Templates)
			if len(choices) == 0 {
				return TargetPlan{}, cliErrors.New(
					cliErrors.BACKEND_NOT_ENABLED,
					i18n.T("deploy.no_compatible_projects"),
				)
			}
			return TargetPlan{ProjectChoices: choices}, nil
		}
	}
	if project == nil {
		return TargetPlan{}, cliErrors.New(
			cliErrors.SUBPROJECT_NOT_FOUND,
			i18n.Tf("deploy.project_not_found", selector),
		).WithContext(map[string]any{
			"selector": selector, "available_projects": workspace.ProjectNames(request.Manifest),
		})
	}

	existingBackend := workspace.DeployForProject(request.Manifest, project.Name).Backend
	if existingBackend != "" && backend == "" {
		return TargetPlan{Targets: []Target{
			manifestProjectTarget(request.ProjectRoot, project, existingBackend),
		}}, nil
	}

	projectTemplate := templateForProject(request.Templates, project)
	compatible := compatibleBackends(s.catalog, projectTemplate)
	if len(compatible) == 0 {
		return TargetPlan{}, cliErrors.New(
			cliErrors.BACKEND_NOT_ENABLED,
			i18n.Tf("deploy.project_not_deployable", project.Name),
		)
	}
	setup := &TargetSetup{
		Project: project, Template: projectTemplate,
		CompatibleBackends: compatible, Backend: backend,
	}
	if backend != "" {
		if _, err := setup.ResolveTarget(request.ProjectRoot, backend); err != nil {
			return TargetPlan{}, err
		}
	}
	return TargetPlan{Setup: setup}, nil
}

// ResolveTarget validates a transport-selected backend and returns the target
// that will be published to the manifest after profile setup succeeds.
func (s TargetSetup) ResolveTarget(projectRoot, backend string) (Target, error) {
	backend = normalizeBackend(backend)
	for _, candidate := range s.CompatibleBackends {
		if candidate == backend {
			return manifestProjectTarget(projectRoot, s.Project, backend), nil
		}
	}
	projectName := ""
	if s.Project != nil {
		projectName = s.Project.Name
	}
	return Target{}, cliErrors.New(
		cliErrors.PROFILE_BACKEND_INVALID,
		i18n.Tf(
			"deploy.provider_incompatible",
			backend,
			projectName,
			strings.Join(s.CompatibleBackends, ", "),
		),
	)
}

func deployableProjectNames(
	backendCatalog *catalog.Catalog,
	manifest *workspace.Manifest,
	registry *template.Registry,
) []string {
	if manifest == nil {
		return nil
	}
	names := make([]string, 0, len(manifest.Projects))
	for index := range manifest.Projects {
		project := &manifest.Projects[index]
		if len(compatibleBackends(backendCatalog, templateForProject(registry, project))) == 0 {
			continue
		}
		names = append(names, project.Name)
	}
	return names
}

func normalizeBackend(value string) string {
	return strings.TrimPrefix(strings.TrimSpace(value), "deploy/")
}
