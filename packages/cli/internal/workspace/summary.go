package workspace

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/i18n"
)

const SummarySchema = "one-cli/workspace-summary/v1"

// Summary is the concise command-line view rendered by a bare `one` inside a
// workspace. It intentionally uses user concepts while retaining stable
// machine-readable values in the JSON/YAML form.
type Summary struct {
	Schema                string           `json:"schema"`
	Name                  string           `json:"name"`
	Root                  string           `json:"root"`
	Projects              []SummaryProject `json:"projects"`
	EnvironmentSource     string           `json:"environment_source"`
	DefaultEnvironment    string           `json:"default_environment"`
	AvailableEnvironments []string         `json:"available_environments"`
	Issues                []SummaryIssue   `json:"issues,omitempty"`
	NextCommand           string           `json:"next_command"`
}

type SummaryProject struct {
	Name                  string `json:"name"`
	RelativeDir           string `json:"relative_dir"`
	CanStartDevelopment   bool   `json:"can_start_development"`
	DependenciesInstalled bool   `json:"dependencies_installed"`
	DeploymentConfigured  bool   `json:"deployment_configured"`
}

type SummaryIssue struct {
	Code    string `json:"code"`
	Project string `json:"project,omitempty"`
}

// BuildSummary reads only workspace and machine-local configuration; it never
// exposes credential values.
func BuildSummary(root string) (Summary, error) {
	m, err := ReadManifest(root)
	if err != nil {
		return Summary{}, err
	}
	name := filepath.Base(root)
	if m.Workspace != nil && strings.TrimSpace(m.Workspace.Name) != "" {
		name = strings.TrimSpace(m.Workspace.Name)
	}
	envSource := EnvBackend(m)
	if envSource == "" {
		envSource = "dotenv"
	}
	defaultEnv := "dev"
	environments := append([]string(nil), DefaultEnvironments...)
	if m.Environments != nil {
		if strings.TrimSpace(m.Environments.Default) != "" {
			defaultEnv = strings.TrimSpace(m.Environments.Default)
		}
		if len(m.Environments.Names) > 0 {
			environments = append([]string(nil), m.Environments.Names...)
		}
	}

	s := Summary{
		Schema:                SummarySchema,
		Name:                  name,
		Root:                  root,
		Projects:              make([]SummaryProject, 0, len(m.Projects)),
		EnvironmentSource:     envSource,
		DefaultEnvironment:    defaultEnv,
		AvailableEnvironments: environments,
	}
	for i := range m.Projects {
		p := &m.Projects[i]
		devCommand := strings.TrimSpace(ProjectDev(m, p.Name))
		projectDir := filepath.Join(root, filepath.FromSlash(p.RelativeDir))
		canDevelop := devCommand != ""
		dependenciesInstalled := ProjectDependenciesInstalled(root, projectDir, p.Toolchain)
		deployConfigured := strings.TrimSpace(DeployForProject(m, p.Name).Backend) != ""
		s.Projects = append(s.Projects, SummaryProject{
			Name:                  p.Name,
			RelativeDir:           p.RelativeDir,
			CanStartDevelopment:   canDevelop,
			DependenciesInstalled: dependenciesInstalled,
			DeploymentConfigured:  deployConfigured,
		})
		if canDevelop && !dependenciesInstalled {
			s.Issues = append(s.Issues, SummaryIssue{Code: "dependencies_not_installed", Project: p.Name})
		}
		if !canDevelop {
			s.Issues = append(s.Issues, SummaryIssue{Code: "development_not_available", Project: p.Name})
		}
		if projectKindFromDir(p.RelativeDir) != ProjectKindPackage && !deployConfigured {
			s.Issues = append(s.Issues, SummaryIssue{Code: "deployment_not_configured", Project: p.Name})
		}
	}

	s.NextCommand = bestNextCommand(s.Projects)
	return s, nil
}

// ProjectDependenciesInstalled reports whether a project can start without a
// package-manager install. Node workspaces share root node_modules; non-Node
// toolchains do not have a separate install phase here.
func ProjectDependenciesInstalled(root, projectDir, toolchain string) bool {
	if strings.TrimSpace(toolchain) != "node" {
		return true
	}
	for _, dir := range []string{filepath.Join(root, "node_modules"), filepath.Join(projectDir, "node_modules")} {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return true
		}
	}
	return false
}

func bestNextCommand(projects []SummaryProject) string {
	if len(projects) == 0 {
		return "one add"
	}
	for _, p := range projects {
		if p.CanStartDevelopment {
			return "one dev " + p.Name
		}
	}
	return "one add"
}

func (s *Summary) RenderTTY(w io.Writer) {
	if s == nil {
		return
	}
	fmt.Fprintf(w, i18n.T("workspace.title")+"\n", s.Name)
	fmt.Fprintln(w, i18n.T("workspace.projects"))
	if len(s.Projects) == 0 {
		fmt.Fprintln(w, i18n.T("workspace.no_projects"))
	}
	for _, p := range s.Projects {
		dev := i18n.T("workspace.dev_unavailable")
		if p.CanStartDevelopment {
			dev = i18n.T("workspace.dev_ready")
			if !p.DependenciesInstalled {
				dev = i18n.T("workspace.dependencies_missing")
			}
		}
		deploy := i18n.T("workspace.deploy_missing")
		if p.DeploymentConfigured {
			deploy = i18n.T("workspace.deploy_ready")
		}
		fmt.Fprintf(w, "  %s  %s  %s\n", p.Name, dev, deploy)
	}
	fmt.Fprintf(w, i18n.T("workspace.environment")+"\n", environmentSourceLabel(s.EnvironmentSource), s.DefaultEnvironment)
	if len(s.Issues) > 0 {
		fmt.Fprintln(w, i18n.T("workspace.issues"))
		for _, issue := range s.Issues {
			fmt.Fprintf(w, "  %s: %s\n", issue.Project, i18n.T("workspace.issue."+issue.Code))
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%s%s\n", i18n.T("workspace.next"), s.NextCommand)
}

func environmentSourceLabel(source string) string {
	if source == EnvBackendDotenv {
		return i18n.T("workspace.env_dotenv")
	}
	return source
}
