package issuetracking

import (
	"context"
	"errors"
	"testing"

	workspaceapp "github.com/torchstellar-team/one-cli/packages/one-core/pkg/application/workspace"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/workspace"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/workspacegroups"
)

type memoryRegistry struct{ document workspace.Registry }

func (r *memoryRegistry) Load(context.Context) (workspace.Registry, error) { return r.document, nil }
func (r *memoryRegistry) Update(_ context.Context, change func(*workspace.Registry) error) error {
	return change(&r.document)
}

type bindingBackend struct {
	Backend
	connected bool
}

func (b *bindingBackend) Status(context.Context) (ConnectionState, error) {
	result := ConnectionState{}
	if b.connected {
		result.Connections = []Connection{{ID: "connection", OrganizationID: "org"}}
	}
	return result, nil
}
func (*bindingBackend) Project(context.Context, string, string) (Project, error) {
	return Project{Named: Named{ID: "remote"}, Teams: []Named{{ID: "team"}}}, nil
}

func TestBindingIsolationAliasesAndDisconnectedAccount(t *testing.T) {
	t.Setenv("ONE_HOME", t.TempDir())
	ctx := context.Background()
	registry, err := workspaceapp.NewRegistryService(&memoryRegistry{document: workspace.Registry{Version: 1, GitRepositoriesImported: true, Workspaces: []workspace.RegistryEntry{{EntryID: "workspace", Root: t.TempDir(), Aliases: []workspace.RegistryAlias{{EntryID: "old"}}}}}})
	if err != nil {
		t.Fatal(err)
	}
	groups := workspacegroups.NewService(registry)
	first, err := groups.Create(ctx, "first")
	if err != nil {
		t.Fatal(err)
	}
	second, err := groups.Create(ctx, "second")
	if err != nil {
		t.Fatal(err)
	}
	backend := &bindingBackend{connected: true}
	service := New(backend, groups)
	input := Binding{Scope: Scope{Kind: "group", ID: first.ID}, ConnectionID: "connection", TeamID: "team", Project: Project{Named: Named{ID: "remote"}}}
	if _, err := service.Bind(ctx, input); err != nil {
		t.Fatal(err)
	}
	other, err := service.Binding(ctx, Scope{Kind: "group", ID: second.ID})
	if err != nil || other != nil {
		t.Fatalf("binding leaked across groups: %+v %v", other, err)
	}
	input.Scope = Scope{Kind: "workspace", ID: "old"}
	if _, err := service.Bind(ctx, input); err != nil {
		t.Fatal(err)
	}
	alias, err := service.Binding(ctx, Scope{Kind: "workspace", ID: "workspace"})
	if err != nil || alias == nil || alias.Scope.ID != "workspace" {
		t.Fatalf("alias did not resolve: %+v %v", alias, err)
	}
	backend.connected = false
	_, err = service.Require(ctx, input.Scope)
	var problem *Error
	if !errors.As(err, &problem) || problem.Code != "connection" {
		t.Fatalf("disconnected account accepted: %v", err)
	}
	if err := service.Unbind(ctx, input.Scope); err != nil {
		t.Fatal(err)
	}
	firstBinding, err := service.Binding(ctx, Scope{Kind: "group", ID: first.ID})
	if err != nil || firstBinding == nil {
		t.Fatal("unbinding workspace removed group binding")
	}
}

type projectCreationBackend struct {
	bindingBackend
	write    bool
	visible  bool
	attempts int
	project  Project
}

func (b *projectCreationBackend) Status(context.Context) (ConnectionState, error) {
	return ConnectionState{Connections: []Connection{{ID: "connection", OrganizationID: "org", CanCreateProjects: b.write}}}, nil
}

func (b *projectCreationBackend) CreateProject(_ context.Context, id string, input ProjectInput) (Project, error) {
	b.attempts++
	b.project = Project{Named: Named{ID: id, Name: input.Name}, Teams: []Named{{ID: input.TeamID}}}
	return Project{}, Fail("network")
}

func (b *projectCreationBackend) Project(_ context.Context, _ string, id string) (Project, error) {
	if b.visible && id == b.project.ID {
		return b.project, nil
	}
	return Project{}, Fail("not_found")
}

func TestCreateAndBindRecoversLostResponse(t *testing.T) {
	t.Setenv("ONE_HOME", t.TempDir())
	ctx := context.Background()
	registry, err := workspaceapp.NewRegistryService(&memoryRegistry{document: workspace.Registry{Version: 1, GitRepositoriesImported: true}})
	if err != nil {
		t.Fatal(err)
	}
	groups := workspacegroups.NewService(registry)
	group, err := groups.Create(ctx, "One project")
	if err != nil {
		t.Fatal(err)
	}
	scope := Scope{Kind: "group", ID: group.ID}
	input := ProjectInput{ConnectionID: "connection", Name: "  One project  ", TeamID: "team"}
	backend := &projectCreationBackend{}
	service := New(backend, groups)
	if _, err := service.CreateAndBind(ctx, scope, input); !hasCode(err, "project_write_scope") || backend.attempts != 0 {
		t.Fatalf("creation without write authorization: %v (%d attempts)", err, backend.attempts)
	}
	backend.write = true
	_, err = service.CreateAndBind(ctx, scope, input)
	var unknown *Error
	if !errors.As(err, &unknown) || unknown.Code != "project_create_unknown" || unknown.OperationID != backend.project.ID {
		t.Fatalf("lost mutation result: %v", err)
	}

	backend.visible = true
	service = New(backend, groups)
	changed := input
	changed.TeamID = "another team"
	if _, err := service.CreateAndBind(ctx, scope, changed); !hasCode(err, "project_create_pending") {
		t.Fatalf("changed pending request accepted: %v", err)
	}
	binding, err := service.CreateAndBind(ctx, scope, input)
	if err != nil || binding.Project.ID != unknown.OperationID || binding.Project.Name != "One project" || binding.TeamID != input.TeamID || backend.attempts != 1 {
		t.Fatalf("creation did not recover: %+v, %v (%d attempts)", binding, err, backend.attempts)
	}
	saved, err := service.Binding(ctx, scope)
	if err != nil || saved == nil || saved.Project.ID != binding.Project.ID {
		t.Fatalf("binding was not persisted: %+v, %v", saved, err)
	}
	if _, err := service.CreateAndBind(ctx, scope, changed); !hasCode(err, "project_bound") || backend.attempts != 1 {
		t.Fatalf("creation replaced an existing binding: %v (%d attempts)", err, backend.attempts)
	}
}
