package issueworkflow

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	api "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/gateway"
	gitmodule "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/git"
	tracking "github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/issuetracking"
)

const testOrgID = "66666666-6666-6666-6666-666666666666"
const testTeamID = "77777777-7777-7777-7777-777777777777"
const testSecondTeamID = "88888888-8888-8888-8888-888888888888"

type mappingLinear struct {
	*fakeLinear
}

func (mappingLinear) Status(context.Context) (tracking.ConnectionState, error) {
	return tracking.ConnectionState{Connections: []tracking.Connection{
		{ID: "gateway-connection", OrganizationID: testOrgID},
	}}, nil
}

func (mappingLinear) Project(context.Context, string, string) (tracking.Project, error) {
	return tracking.Project{Teams: []tracking.Named{{ID: testTeamID}, {ID: testSecondTeamID}}}, nil
}

func (mappingLinear) Team(context.Context, string, string) (tracking.Team, error) {
	return tracking.Team{States: []tracking.State{
		{Named: tracking.Named{ID: "99999999-9999-9999-9999-999999999999", Name: "In Progress"}, Type: "started"},
		{Named: tracking.Named{ID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", Name: "In Review"}, Type: "started"},
		{Named: tracking.Named{ID: "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", Name: "Done"}, Type: "completed"},
	}}, nil
}

type mappingCatalog struct {
	repo         api.RepositoryOption
	choicesCalls int
}

func (c *mappingCatalog) List(context.Context) ([]api.RepositoryOption, error) {
	return []api.RepositoryOption{c.repo}, nil
}

func (*mappingCatalog) Validate(context.Context, api.RepositoryOption) error { return nil }

func (c *mappingCatalog) Choices(context.Context, api.RepositoryOption) (api.RepositoryChoices, error) {
	c.choicesCalls++
	return api.RepositoryChoices{Base: "main", Branches: []string{"main"}, Tasks: []string{"//:check"}}, nil
}

func TestProjectConfigurationPersistsAndEnablesDispatch(t *testing.T) {
	t.Run("legacy GitHub", func(t *testing.T) {
		verifyProjectConfiguration(t, api.RepositoryOption{GitHub: "org/example"})
	})
	t.Run("Gitea", func(t *testing.T) {
		verifyProjectConfiguration(t, api.RepositoryOption{
			Repository: gitmodule.RepositoryRef{Host: "git.example.com", Slug: "org/example"},
		})
	})
}

func verifyProjectConfiguration(t *testing.T, repo api.RepositoryOption) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gateway.json")
	state := t.TempDir()
	cfg := Config{StateDir: state, CodexReadRoots: []string{state}}
	payload, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, payload, 0600); err != nil {
		t.Fatal(err)
	}
	store, err := OpenStore(state)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	service, f, _, _, _ := fixture(store)
	f.issue.Team.ID = testTeamID
	service.projects = nil
	linear := mappingLinear{fakeLinear: f}
	service.linear = linear
	repo.ID, repo.Name, repo.Path = repositoryID(state), "Example", state
	catalog := &mappingCatalog{repo: repo}
	service.manager = &ProjectManager{Path: path, Accounts: linear, Catalog: catalog}
	repositoriesReq := api.WorkRequest{Operation: api.WorkRepositories}
	mappingReq := api.WorkRequest{Operation: api.WorkMapping, ProjectID: projectID}
	repos := verifyWorkRequest(t, service, repositoriesReq, http.StatusOK)
	if !bytes.Contains(repos, []byte(repo.ID)) {
		t.Fatal("known repository was missing from the catalog")
	}
	if result := verifyWorkRequest(t, service, mappingReq, http.StatusOK); string(bytes.TrimSpace(result)) != "null" {
		t.Fatalf("unconfigured project returned %s", result)
	}
	input := api.ProjectInput{
		OrganizationID: testOrgID, ConnectionID: "desktop-connection", RepositoryID: repo.ID,
		TeamID: testTeamID,
	}
	saveReq := api.WorkRequest{Operation: api.WorkSaveMapping, ProjectID: projectID, Mapping: input}
	verifyWorkRequest(t, service, saveReq, http.StatusOK)
	initial, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	bound := initial.Projects[projectID]
	if catalog.choicesCalls != 0 || bound.Base != "" || len(bound.Check) != 0 {
		t.Fatalf("binding required branch or check resolution: %+v", bound)
	}
	wantFlow := api.TeamFlow{Running: "99999999-9999-9999-9999-999999999999", Review: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", Done: "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"}
	if got := bound.Teams[testTeamID]; got != wantFlow {
		t.Fatalf("binding did not use the existing Linear workflow: %+v", got)
	}
	saveReq.Mapping.Base = "main"
	saveReq.Mapping.CheckTask = "-c arbitrary"
	verifyWorkRequest(t, service, saveReq, http.StatusUnprocessableEntity)
	if current, _ := service.project(projectID); current.Base != "" || len(current.Check) != 0 {
		t.Fatal("invalid check changed the configured projects")
	}
	saveReq.Mapping.CheckTask = "//:check"
	customFlow := wantFlow
	customFlow.Running, customFlow.Review = customFlow.Review, customFlow.Running
	saveReq.Mapping.Flow = customFlow
	verifyWorkRequest(t, service, saveReq, http.StatusOK)
	if !service.list(projectID).Enabled {
		t.Fatal("configuration did not take effect for job dispatch")
	}
	statusBody := verifyWorkRequest(t, service, api.WorkRequest{Operation: api.WorkStatus}, http.StatusOK)
	var snapshot api.Snapshot
	wantMappings := []api.ProjectSummary{{ProjectID: projectID, GitHub: repo.GitHub, Repository: repo.Repository, Base: "main"}}
	if err := json.Unmarshal(statusBody, &snapshot); err != nil || snapshot.Projects != 1 ||
		!reflect.DeepEqual(snapshot.Mappings, wantMappings) || bytes.Contains(statusBody, []byte("gateway-connection")) {
		t.Fatalf("unexpected public mapping summary: %s, %v", statusBody, err)
	}
	input.TeamID = testSecondTeamID
	saveReq.Mapping = input
	verifyWorkRequest(t, service, saveReq, http.StatusOK)
	input.TeamID = testTeamID
	saveReq.Mapping = input
	verifyWorkRequest(t, service, saveReq, http.StatusOK)
	saved, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	project := saved.Projects[projectID]
	if len(project.Teams) != 2 || project.ConnectionID != "gateway-connection" || project.ref() != repo.Ref() ||
		project.Base != "main" || project.Teams[testTeamID] != customFlow || project.Teams[testSecondTeamID] != wantFlow ||
		!slices.Equal(project.Check, checkCommand("//:check")) || !slices.Equal(saved.CodexReadRoots, cfg.CodexReadRoots) {
		t.Fatalf("saved config did not preserve the mapping: %+v", saved)
	}
	verifyWorkRequest(t, service, api.WorkRequest{Operation: api.WorkDispatch, ProjectID: projectID, IssueID: issueID}, http.StatusAccepted)
}
