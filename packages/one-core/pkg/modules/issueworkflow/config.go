package issueworkflow

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	api "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
	gitmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/git"
	tracking "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/issuetracking"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

type Config struct {
	StateDir       string             `json:"stateDir"`
	CodexReadRoots []string           `json:"codexReadRoots"`
	Projects       map[string]Project `json:"projects"`
}

type Project struct {
	Root         string                  `json:"root,omitempty"`
	GitHub       string                  `json:"github,omitempty"`
	Repository   gitmodule.RepositoryRef `json:"repository,omitzero"`
	Base         string                  `json:"base,omitempty"`
	Check        []string                `json:"check,omitempty"`
	ConnectionID string                  `json:"connectionId"`
	Teams        map[string]TeamFlow     `json:"teams"`
	Repositories []ProjectRepository     `json:"repositories,omitempty"`
}

type ProjectRepository struct {
	Root       string                  `json:"root"`
	GitHub     string                  `json:"github,omitempty"`
	Repository gitmodule.RepositoryRef `json:"repository,omitzero"`
	Base       string                  `json:"base,omitempty"`
	Check      []string                `json:"check,omitempty"`
}

func (r ProjectRepository) project() Project {
	return Project{Root: r.Root, GitHub: r.GitHub, Repository: r.Repository, Base: r.Base, Check: r.Check}
}

func (p Project) repositorySettings() []ProjectRepository {
	if len(p.Repositories) != 0 {
		return p.Repositories
	}
	if p.Root == "" {
		return nil
	}
	return []ProjectRepository{{Root: p.Root, GitHub: p.GitHub, Repository: p.Repository, Base: p.Base, Check: p.Check}}
}

func (p Project) validRepositories() bool {
	if len(p.Repositories) != 0 && (p.Root != "" || p.GitHub != "" || p.Repository != (gitmodule.RepositoryRef{}) || p.Base != "" || len(p.Check) != 0) {
		return false
	}
	settings := p.repositorySettings()
	if len(settings) == 0 {
		return false
	}
	roots, refs := map[string]bool{}, map[string]bool{}
	for _, setting := range settings {
		ref := setting.project().ref()
		root := filepath.Clean(setting.Root)
		key := ref.Host + "/" + strings.ToLower(ref.Slug)
		if !filepath.IsAbs(setting.Root) || !ref.Valid() || (setting.GitHub != "" && setting.Repository != (gitmodule.RepositoryRef{})) ||
			(setting.Base != "" && !branchPattern.MatchString(setting.Base)) || (len(setting.Check) > 0 && setting.Check[0] == "") || roots[root] || refs[key] {
			return false
		}
		roots[root], refs[key] = true, true
	}
	return true
}

type TeamFlow = api.TeamFlow

var uuidPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}(-[0-9a-f]{4}){3}-[0-9a-f]{12}$`)
var branchPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9/_.-]*$`)

func LoadConfig(path string) (Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return Config{}, err
	}
	defer file.Close()
	var cfg Config
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return Config{}, i18n.Errorf("linear.cli.workflow_config_trailing")
	}
	if !filepath.IsAbs(cfg.StateDir) {
		return Config{}, i18n.Errorf("linear.cli.workflow_config_path", "stateDir")
	}
	for _, root := range cfg.CodexReadRoots {
		if !filepath.IsAbs(root) {
			return Config{}, i18n.Errorf("linear.cli.workflow_config_path", "codexReadRoots")
		}
	}
	for id, p := range cfg.Projects {
		if !uuidPattern.MatchString(id) || !p.validRepositories() || p.ConnectionID == "" || len(p.Teams) == 0 {
			return Config{}, i18n.Errorf("linear.cli.workflow_config_project", id)
		}
		for teamID, flow := range p.Teams {
			if !uuidPattern.MatchString(teamID) || !uuidPattern.MatchString(flow.Running) ||
				!uuidPattern.MatchString(flow.Review) || !uuidPattern.MatchString(flow.Done) ||
				flow.Running == flow.Done || flow.Review == flow.Done {
				return Config{}, i18n.Errorf("linear.cli.workflow_config_team", teamID)
			}
		}
	}
	return cfg, nil
}

// SetupMapping exports project settings that can be applied on another machine.
func (cfg Config) SetupMapping(id string) (api.SetupMapping, error) {
	project, exists := cfg.Projects[id]
	if !exists {
		return api.SetupMapping{}, i18n.Errorf("gateway.cli.setup_project_missing", id)
	}
	organization, _, _ := strings.Cut(project.ConnectionID, ":")
	result := api.SetupMapping{ID: id, Organization: organization, Teams: project.Teams}
	settings := project.repositorySettings()
	for _, setting := range settings {
		result.Repositories = append(result.Repositories, setting.project().ref())
		if len(settings) == 1 {
			result.Base = setting.Base
			if len(setting.Check) == 3 && slices.Equal(setting.Check[:2], []string{"mise", "run"}) {
				result.CheckTask = setting.Check[2]
				continue
			}
		}
		if (len(settings) != 1 && setting.Base != "") || len(setting.Check) != 0 {
			return api.SetupMapping{}, i18n.Errorf("gateway.cli.setup_project_check", id)
		}
	}
	return result, nil
}

func (p Project) ref() gitmodule.RepositoryRef {
	return api.RepositoryOption{GitHub: p.GitHub, Repository: p.Repository}.Ref()
}

func (p Project) binding(project, team string) tracking.Binding {
	return tracking.Binding{
		ConnectionID: p.ConnectionID, TeamID: team,
		Project: tracking.Project{Named: tracking.Named{ID: project}},
	}
}

func matchesTeamFlow(f TeamFlow, team tracking.Team) bool {
	stateType := func(id string) string {
		index := slices.IndexFunc(team.States, func(state tracking.State) bool { return state.ID == id })
		if index < 0 {
			return ""
		}
		return team.States[index].Type
	}
	return stateType(f.Running) == "started" && stateType(f.Review) == "started" && stateType(f.Done) == "completed"
}
