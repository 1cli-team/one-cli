package issueworkflow

import (
	"context"
	"encoding/json"
	"maps"
	"regexp"
	"slices"
	"sort"

	api "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
	gitmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/git"
	tracking "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/issuetracking"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/fsutil"
)

type projectAccounts interface {
	Status(context.Context) (tracking.ConnectionState, error)
	Project(context.Context, string, string) (tracking.Project, error)
}

type projectCatalog interface {
	List(context.Context) ([]api.RepositoryOption, error)
	Validate(context.Context, api.RepositoryOption) error
	Choices(context.Context, api.RepositoryOption) (api.RepositoryChoices, error)
}

type ProjectManager struct {
	Path     string
	Accounts projectAccounts
	Catalog  projectCatalog
}

func (s *Service) ManageProjects(path string, accounts projectAccounts, git *gitmodule.Service) {
	s.manager = &ProjectManager{Path: path, Accounts: accounts, Catalog: RepositoryCatalog{Git: git}}
}

func checkCommand(task string) []string {
	return []string{"mise", "run", task}
}

func projectMapping(p Project) api.ProjectMapping {
	if len(p.Repositories) != 0 {
		ids := make([]string, 0, len(p.Repositories))
		for _, repo := range p.Repositories {
			ids = append(ids, repositoryID(repo.Root))
		}
		return api.ProjectMapping{RepositoryIDs: ids, Teams: p.Teams}
	}
	result := api.ProjectMapping{RepositoryID: repositoryID(p.Root), Base: p.Base, Teams: p.Teams, ExistingCheck: len(p.Check) > 0}
	if len(p.Check) >= 3 && slices.Equal(p.Check, checkCommand(p.Check[2])) {
		result.CheckTask = p.Check[2]
	}
	return result
}

func (s *Service) repositories(ctx context.Context) ([]api.RepositoryOption, error) {
	results, err := s.manager.Catalog.List(ctx)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]api.RepositoryOption, len(results))
	for _, repo := range results {
		byID[repo.ID] = repo
	}
	s.configMu.RLock()
	for _, p := range s.projects {
		for _, repo := range p.repositorySettings() {
			id := repositoryID(repo.Root)
			if _, found := byID[id]; !found {
				byID[id] = api.RepositoryOption{ID: id, Name: repo.project().ref().Slug, Path: repo.Root, GitHub: repo.GitHub, Repository: repo.Repository}
			}
		}
	}
	s.configMu.RUnlock()
	results = slices.Collect(maps.Values(byID))
	sort.Slice(results, func(i, j int) bool {
		if results[i].Name == results[j].Name {
			return results[i].Path < results[j].Path
		}
		return results[i].Name < results[j].Name
	})
	return results, nil
}

func (s *Service) repositoryOption(ctx context.Context, id string) (api.RepositoryOption, error) {
	repos, err := s.repositories(ctx)
	if err != nil {
		return api.RepositoryOption{}, err
	}
	for _, repo := range repos {
		if repo.ID == id {
			return repo, nil
		}
	}
	return api.RepositoryOption{}, nil
}

func (s *Service) linearConnection(ctx context.Context, input api.ProjectInput) (string, error) {
	status, err := s.manager.Accounts.Status(ctx)
	if err != nil {
		return "", err
	}
	var matched []string
	for _, conn := range status.Connections {
		if conn.OrganizationID != input.OrganizationID {
			continue
		}
		if conn.ID == input.ConnectionID {
			return conn.ID, nil
		}
		matched = append(matched, conn.ID)
	}
	if len(matched) == 1 {
		return matched[0], nil
	}
	return "", nil
}

func (s *Service) saveMapping(ctx context.Context, id string, input api.ProjectInput) (api.ProjectMapping, string, error) {
	ids := input.RepositoryIDs
	if len(ids) == 0 {
		ids = []string{input.RepositoryID}
	}
	if !uuidPattern.MatchString(input.OrganizationID) || !uuidPattern.MatchString(input.TeamID) ||
		(input.Base != "" && !branchPattern.MatchString(input.Base)) ||
		(len(ids) > 1 && (input.Base != "" || input.CheckTask != "")) {
		return api.ProjectMapping{}, "invalid_mapping", nil
	}
	for _, repoID := range ids {
		if !repositoryPattern.MatchString(repoID) {
			return api.ProjectMapping{}, "invalid_mapping", nil
		}
	}
	catalog, err := s.repositories(ctx)
	if err != nil {
		return api.ProjectMapping{}, "repository_unavailable", err
	}
	byID := make(map[string]api.RepositoryOption, len(catalog))
	for _, repo := range catalog {
		byID[repo.ID] = repo
	}
	var repos []api.RepositoryOption
	seen := make(map[string]bool, len(ids))
	for _, repoID := range ids {
		repo, ok := byID[repoID]
		if !ok {
			return api.ProjectMapping{}, "repository_unavailable", nil
		}
		if seen[repoID] {
			return api.ProjectMapping{}, "invalid_mapping", nil
		}
		seen[repoID] = true
		if input.Base != "" || input.CheckTask != "" {
			choices, err := s.manager.Catalog.Choices(ctx, repo)
			if err != nil {
				return api.ProjectMapping{}, "repository_unavailable", err
			}
			if input.Base != "" && !slices.Contains(choices.Branches, input.Base) {
				return api.ProjectMapping{}, "invalid_branch", nil
			}
			if input.CheckTask != "" && !slices.Contains(choices.Tasks, input.CheckTask) {
				return api.ProjectMapping{}, "invalid_check", nil
			}
		} else if err := s.manager.Catalog.Validate(ctx, repo); err != nil {
			return api.ProjectMapping{}, "repository_unavailable", err
		}
		repos = append(repos, repo)
	}
	connection, err := s.linearConnection(ctx, input)
	if err != nil || connection == "" {
		return api.ProjectMapping{}, "linear_connection", err
	}
	project, err := s.manager.Accounts.Project(ctx, connection, id)
	if err != nil {
		return api.ProjectMapping{}, "linear_project", err
	}
	if !slices.ContainsFunc(project.Teams, func(team tracking.Named) bool { return team.ID == input.TeamID }) {
		return api.ProjectMapping{}, "invalid_team", nil
	}
	team, err := s.linear.Team(ctx, connection, input.TeamID)
	if err != nil {
		return api.ProjectMapping{}, "linear_project", err
	}
	return s.persistMapping(id, repos, input, connection, team)
}

func (s *Service) persistMapping(id string, repos []api.RepositoryOption, input api.ProjectInput, connection string, team tracking.Team) (api.ProjectMapping, string, error) {
	s.configMu.Lock()
	defer s.configMu.Unlock()
	cfg, err := LoadConfig(s.manager.Path)
	if err != nil {
		return api.ProjectMapping{}, "config_write", err
	}
	old := cfg.Projects[id]
	flow := input.Flow
	if flow == (TeamFlow{}) {
		// Reuse the previously saved mapping if the bound team still has those states.
		flow = old.Teams[input.TeamID]
		if !matchesTeamFlow(flow, team) {
			flow = defaultTeamFlow(team)
		}
		if !matchesTeamFlow(flow, team) {
			return api.ProjectMapping{}, "workflow_states", nil
		}
	} else if !matchesTeamFlow(flow, team) {
		return api.ProjectMapping{}, "invalid_states", nil
	}
	settings := make([]ProjectRepository, 0, len(repos))
	previous := old.repositorySettings()
	for _, repo := range repos {
		setting := ProjectRepository{Root: repo.Path, GitHub: repo.GitHub, Repository: repo.Repository, Base: input.Base}
		for _, prior := range previous {
			if repositoryID(prior.Root) == repo.ID && prior.project().ref().Matches(repo.Ref()) {
				if setting.Base == "" {
					setting.Base = prior.Base
				}
				setting.Check = prior.Check
				break
			}
		}
		if input.CheckTask != "" {
			setting.Check = checkCommand(input.CheckTask)
		}
		settings = append(settings, setting)
	}
	teams := maps.Clone(old.Teams)
	if teams == nil {
		teams = make(map[string]TeamFlow)
	}
	teams[input.TeamID] = flow
	project := Project{Repositories: settings}
	if len(settings) == 1 {
		project = settings[0].project()
	}
	project.ConnectionID, project.Teams = connection, teams
	if !project.validRepositories() {
		return api.ProjectMapping{}, "invalid_mapping", nil
	}
	if cfg.Projects == nil {
		cfg.Projects = make(map[string]Project)
	}
	cfg.Projects[id] = project
	payload, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return api.ProjectMapping{}, "config_write", err
	}
	if err := fsutil.WriteAtomic(s.manager.Path, append(payload, '\n'), 0600); err != nil {
		return api.ProjectMapping{}, "config_write", err
	}
	s.projects = cfg.Projects
	return projectMapping(project), "", nil
}

var repositoryPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)
