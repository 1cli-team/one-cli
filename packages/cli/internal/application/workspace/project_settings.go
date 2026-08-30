package workspace

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	catalog "github.com/torchstellar-team/one-cli/packages/cli/internal/core/backend"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/profile"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/core/template"
	workspacecore "github.com/torchstellar-team/one-cli/packages/cli/internal/core/workspace"
)

// ProjectSettingsSchema versions the safe, project-focused Dashboard
// projection. It deliberately contains no profile values or credentials.
const ProjectSettingsSchema = "one-cli/workspace-project/v1"

type ProjectSettings struct {
	Schema      string                 `json:"schema"`
	Root        string                 `json:"root"`
	Environment string                 `json:"environment"`
	Project     ProjectSettingsProject `json:"project"`
}

type ProjectSettingsProject struct {
	Name                  string                     `json:"name"`
	RelativeDir           string                     `json:"relativeDir"`
	Kind                  string                     `json:"kind"`
	TemplateID            string                     `json:"templateId,omitempty"`
	Toolchain             string                     `json:"toolchain,omitempty"`
	PackageManager        string                     `json:"packageManager,omitempty"`
	BuildVersion          string                     `json:"buildVersion,omitempty"`
	DevCommand            string                     `json:"devCommand,omitempty"`
	DefaultEnvironment    string                     `json:"defaultEnvironment,omitempty"`
	AvailableEnvironments []string                   `json:"availableEnvironments"`
	Environment           ProjectEnvironmentSettings `json:"environment"`
	Container             ProjectContainerSettings   `json:"container"`
	Deploy                ProjectDeploySettings      `json:"deploy"`
}

type ProjectProfileRef struct {
	Name   string `json:"name"`
	Source string `json:"source"`
}

type ProjectEnvironmentSettings struct {
	Backend         string             `json:"backend,omitempty"`
	Path            string             `json:"path,omitempty"`
	Inherits        bool               `json:"inherits"`
	Disabled        bool               `json:"disabled"`
	Keys            []string           `json:"keys"`
	SelectedProfile string             `json:"selectedProfile"`
	Profile         *ProjectProfileRef `json:"profile,omitempty"`
}

type ProjectContainerSettings struct {
	Enabled         bool               `json:"enabled"`
	Backend         string             `json:"backend,omitempty"`
	Image           string             `json:"image,omitempty"`
	Namespace       string             `json:"namespace,omitempty"`
	SelectedProfile string             `json:"selectedProfile"`
	Profile         *ProjectProfileRef `json:"profile,omitempty"`
}

type ProjectDeploySettings struct {
	Backend           string             `json:"backend,omitempty"`
	CompatibleTargets []string           `json:"compatibleTargets"`
	Config            map[string]any     `json:"config"`
	SelectedProfile   string             `json:"selectedProfile"`
	Profile           *ProjectProfileRef `json:"profile,omitempty"`
}

// ProjectSettings returns the manifest-owned settings for one project plus
// safe machine-profile references. Profile values are never copied into the
// response.
func (s *Service) ProjectSettings(
	ctx context.Context,
	root, projectName, environment string,
) (ProjectSettings, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.projectSettings(ctx, root, projectName, environment)
}

func (s *Service) projectSettings(
	ctx context.Context,
	root, projectName, environment string,
) (ProjectSettings, error) {
	manifest, err := workspacecore.ReadManifest(root)
	if err != nil {
		return ProjectSettings{}, err
	}
	project := findProject(manifest, strings.TrimSpace(projectName))
	if project == nil {
		return ProjectSettings{}, fmt.Errorf("%w: %s", ErrProjectNotFound, projectName)
	}
	environment, err = validateEnvironment(manifest, environment)
	if err != nil {
		return ProjectSettings{}, err
	}
	registry, err := template.Fetch(ctx, "")
	if err != nil {
		return ProjectSettings{}, err
	}
	compatible := projectCompatibleDeployTargets(registry, project.TemplateID)
	environments, defaultEnvironment := projectEnvironments(manifest)
	profileEnvironment := workspacecore.ProfileBindingEnvironment(manifest, environment)

	env := ProjectEnvironmentSettings{
		Backend:  strings.TrimSpace(workspacecore.EnvBackend(manifest)),
		Inherits: true,
		Keys:     []string{},
	}
	if override := workspacecore.ProjectEnv(manifest, project.Name); override != nil {
		env.Path = override.Path
		env.Disabled = override.Disabled
		if override.Inherits != nil {
			env.Inherits = *override.Inherits
		}
		env.Keys = append([]string(nil), override.Keys...)
		sort.Strings(env.Keys)
	}
	if env.Backend != "" {
		env.Profile = s.resolveProfileRef(
			manifest, root, profileEnvironment, project.Name, profile.DomainEnv, env.Backend,
		)
		env.SelectedProfile, err = s.directProfileSelection(
			root, project.Name, profileEnvironment, profile.DomainEnv, env.Backend, env.Profile,
		)
		if err != nil {
			return ProjectSettings{}, err
		}
	}

	containerEnabled, containerImage := workspacecore.ContainerForProject(manifest, project.Name)
	container := ProjectContainerSettings{
		Enabled:   containerEnabled,
		Backend:   workspacecore.ContainerKindForProject(manifest, project.Name),
		Image:     containerImage,
		Namespace: workspacecore.ContainerNamespaceForProject(manifest, project.Name),
	}
	if container.Enabled && container.Backend != "" {
		container.Profile = s.resolveProfileRef(
			manifest, root, profileEnvironment, project.Name, profile.DomainContainer, container.Backend,
		)
		container.SelectedProfile, err = s.directProfileSelection(
			root, project.Name, profileEnvironment, profile.DomainContainer, container.Backend,
			container.Profile,
		)
		if err != nil {
			return ProjectSettings{}, err
		}
	}

	deployBackend := effectiveDeployBackend(manifest, project.Name)
	deployConfig := map[string]any{}
	if deployBackend != "" {
		spec, ok := s.catalog.Lookup(catalog.DomainDeploy, deployBackend)
		if !ok {
			return ProjectSettings{}, fmt.Errorf("workspace: unknown deploy backend %q in manifest", deployBackend)
		}
		deployConfig, err = safeProjectConfig(
			workspacecore.DeployConfigRawForProject(manifest, project.Name), spec.Project.Fields,
		)
		if err != nil {
			return ProjectSettings{}, fmt.Errorf("workspace: project %q deploy config: %w", project.Name, err)
		}
	}
	deploy := ProjectDeploySettings{
		Backend:           deployBackend,
		CompatibleTargets: compatible,
		Config:            deployConfig,
	}
	if deploy.Backend != "" {
		deploy.Profile = s.resolveProfileRef(
			manifest, root, profileEnvironment, project.Name, profile.DomainDeploy, deploy.Backend,
		)
		deploy.SelectedProfile, err = s.directProfileSelection(
			root, project.Name, profileEnvironment, profile.DomainDeploy, deploy.Backend,
			deploy.Profile,
		)
		if err != nil {
			return ProjectSettings{}, err
		}
	}

	return ProjectSettings{
		Schema:      ProjectSettingsSchema,
		Root:        root,
		Environment: environment,
		Project: ProjectSettingsProject{
			Name:                  project.Name,
			RelativeDir:           project.RelativeDir,
			Kind:                  projectKind(project.RelativeDir),
			TemplateID:            project.TemplateID,
			Toolchain:             project.Toolchain,
			PackageManager:        project.PackageManager,
			BuildVersion:          project.BuildVersion,
			DevCommand:            workspacecore.ProjectDev(manifest, project.Name),
			DefaultEnvironment:    defaultEnvironment,
			AvailableEnvironments: environments,
			Environment:           env,
			Container:             container,
			Deploy:                deploy,
		},
	}, nil
}

func (s *Service) resolveProfileRef(
	manifest *workspacecore.Manifest,
	root, environment, projectName string,
	domain profile.Domain,
	backend string,
) *ProjectProfileRef {
	if s.profiles == nil || strings.TrimSpace(backend) == "" {
		return nil
	}
	resolved, err := s.profiles.Resolve(profile.ResolveInput{
		Domain:        domain,
		Backend:       backend,
		WorkspaceID:   workspacecore.WorkspaceID(manifest),
		WorkspaceRoot: root,
		ProjectName:   projectName,
		Environment:   environment,
	})
	if err != nil || resolved == nil || strings.TrimSpace(resolved.Name) == "" {
		return nil
	}
	return &ProjectProfileRef{Name: resolved.Name, Source: resolved.Source}
}

func (s *Service) directProfileSelection(
	root, projectName, environment string,
	domain profile.Domain,
	backend string,
	effective *ProjectProfileRef,
) (string, error) {
	if environment == "" {
		directSource := workspaceDirectSource(environment)
		if projectName != "" {
			directSource = projectDirectSource(environment)
		}
		return directProfileName(effective, directSource), nil
	}
	if s.profiles == nil {
		return "", nil
	}
	return s.profiles.EnvironmentProfileBinding(
		root, projectName, environment, domain, backend,
	)
}

func projectDirectSource(environment string) string {
	if environment != "" {
		return "workspace-project-environment"
	}
	return "workspace-project"
}

func directProfileName(resolved *ProjectProfileRef, directSource string) string {
	if resolved == nil || resolved.Source != directSource {
		return ""
	}
	return resolved.Name
}

func projectKind(relativeDir string) string {
	dir := strings.TrimPrefix(strings.TrimSpace(relativeDir), "./")
	switch {
	case dir == "services" || strings.HasPrefix(dir, "services/"):
		return workspacecore.ProjectKindService
	case dir == "packages" || strings.HasPrefix(dir, "packages/"):
		return workspacecore.ProjectKindPackage
	default:
		return workspacecore.ProjectKindApp
	}
}

func projectCompatibleDeployTargets(registry *template.Registry, templateID string) []string {
	if registry == nil {
		return []string{}
	}
	for _, entry := range registry.Templates {
		if entry.ID == templateID {
			out := append([]string(nil), entry.Compat[string(catalog.DomainDeploy)]...)
			if out == nil {
				return []string{}
			}
			return out
		}
	}
	return []string{}
}

func projectEnvironments(manifest *workspacecore.Manifest) ([]string, string) {
	environments := append([]string(nil), workspacecore.DefaultEnvironments...)
	defaultEnvironment := ""
	if manifest != nil && manifest.Environments != nil {
		if len(manifest.Environments.Names) > 0 {
			environments = append([]string(nil), manifest.Environments.Names...)
		}
		defaultEnvironment = strings.TrimSpace(manifest.Environments.Default)
	}
	if defaultEnvironment == "" && len(environments) > 0 {
		defaultEnvironment = environments[0]
	}
	return environments, defaultEnvironment
}

func effectiveDeployBackend(manifest *workspacecore.Manifest, projectName string) string {
	if selected := workspacecore.DeployForProject(manifest, projectName).Backend; selected != "" {
		return strings.TrimSpace(selected)
	}
	if manifest != nil && manifest.Domains != nil && manifest.Domains.Deploy != nil {
		return strings.TrimSpace(manifest.Domains.Deploy.Kind)
	}
	return ""
}

// safeProjectConfig projects only catalog-declared fields out of a manifest
// config object. Unknown keys are omitted rather than reflected, which makes
// this read path safe even when a hand-edited manifest accidentally contains
// a token-like field.
func safeProjectConfig(raw json.RawMessage, fields []catalog.ProjectFieldSpec) (map[string]any, error) {
	out := map[string]any{}
	if len(bytes.TrimSpace(raw)) == 0 {
		return out, nil
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil || object == nil || bytes.TrimSpace(raw)[0] != '{' {
		if err == nil {
			err = fmt.Errorf("must be a JSON object")
		}
		return nil, err
	}
	for _, field := range fields {
		value, ok := rawValueAtPath(object, field.Path)
		if !ok {
			continue
		}
		var text string
		if err := json.Unmarshal(value, &text); err != nil {
			// A wrong-type hand edit is not safe form data; omit it without
			// reflecting arbitrary nested JSON to the Dashboard.
			continue
		}
		setValueAtPath(out, field.Path, text)
	}
	return out, nil
}

func rawValueAtPath(object map[string]json.RawMessage, path string) (json.RawMessage, bool) {
	parts := strings.Split(path, "/")
	current := object
	for index, part := range parts {
		raw, ok := current[part]
		if !ok {
			return nil, false
		}
		if index == len(parts)-1 {
			return raw, true
		}
		var child map[string]json.RawMessage
		if err := json.Unmarshal(raw, &child); err != nil {
			return nil, false
		}
		current = child
	}
	return nil, false
}

func setValueAtPath(object map[string]any, path string, value any) {
	parts := strings.Split(path, "/")
	current := object
	for _, part := range parts[:len(parts)-1] {
		next, ok := current[part].(map[string]any)
		if !ok {
			next = map[string]any{}
			current[part] = next
		}
		current = next
	}
	current[parts[len(parts)-1]] = value
}
