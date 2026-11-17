package issuetracking

import (
	"context"
	"slices"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/modules/workspacegroups"
)

type Service struct {
	Backend Backend
	groups  *workspacegroups.Service
	store   *bindingStore
}

func New(backend Backend, groups *workspacegroups.Service) *Service {
	return &Service{Backend: backend, groups: groups, store: &bindingStore{}}
}

// Resolve pins a binding to a logical project, including aliases for moved workspaces.
func (s *Service) Resolve(ctx context.Context, scope Scope) (Scope, error) {
	if scope.ID == "" || (scope.Kind != "group" && scope.Kind != "workspace") {
		return Scope{}, Fail("scope")
	}
	listing, err := s.groups.List(ctx)
	if err != nil {
		return Scope{}, err
	}
	if scope.Kind == "group" {
		for _, group := range listing.Groups {
			if group.ID == scope.ID {
				return scope, nil
			}
		}
	} else {
		id, _, _ := strings.Cut(scope.ID, "~")
		for _, workspace := range listing.Workspaces {
			if workspace.EntryID == id {
				return Scope{Kind: "workspace", ID: workspace.EntryID}, nil
			}
			for _, alias := range workspace.Aliases {
				if alias.EntryID == id {
					return Scope{Kind: "workspace", ID: workspace.EntryID}, nil
				}
			}
		}
	}
	return Scope{}, Fail("scope")
}

func (s *Service) Binding(ctx context.Context, scope Scope) (*Binding, error) {
	resolved, err := s.Resolve(ctx, scope)
	if err != nil {
		return nil, err
	}
	return s.store.get(ctx, resolved)
}

func (s *Service) Bind(ctx context.Context, input Binding) (Binding, error) {
	scope, err := s.Resolve(ctx, input.Scope)
	if err != nil {
		return Binding{}, err
	}
	connection, err := s.connection(ctx, input.ConnectionID)
	if err != nil {
		return Binding{}, err
	}
	project, err := s.Backend.Project(ctx, connection.ID, input.Project.ID)
	if err != nil {
		return Binding{}, err
	}
	binding, err := newBinding(scope, connection, project, input.TeamID)
	if err != nil {
		return Binding{}, err
	}
	if err := s.store.put(ctx, scope, &binding); err != nil {
		return Binding{}, err
	}
	return binding, nil
}

func (s *Service) connection(ctx context.Context, id string) (Connection, error) {
	status, err := s.Backend.Status(ctx)
	if err != nil {
		return Connection{}, err
	}
	for _, candidate := range status.Connections {
		if candidate.ID == id {
			return candidate, nil
		}
	}
	return Connection{}, Fail("connection")
}

func newBinding(scope Scope, connection Connection, project Project, teamID string) (Binding, error) {
	if !slices.ContainsFunc(project.Teams, func(team Named) bool { return team.ID == teamID }) {
		return Binding{}, Fail("team")
	}
	return Binding{Scope: scope, ConnectionID: connection.ID, OrganizationID: connection.OrganizationID, Project: project, TeamID: teamID}, nil
}

func (s *Service) Unbind(ctx context.Context, scope Scope) error {
	resolved, err := s.Resolve(ctx, scope)
	if err != nil {
		return err
	}
	return s.store.put(ctx, resolved, nil)
}

func (s *Service) Require(ctx context.Context, scope Scope) (Binding, error) {
	binding, err := s.Binding(ctx, scope)
	if err != nil {
		return Binding{}, err
	}
	if binding == nil {
		return Binding{}, Fail("binding")
	}
	status, err := s.Backend.Status(ctx)
	if err != nil {
		return Binding{}, err
	}
	for _, connection := range status.Connections {
		if connection.ID == binding.ConnectionID && connection.OrganizationID == binding.OrganizationID {
			return *binding, nil
		}
	}
	return Binding{}, Fail("connection")
}

// ProjectBinding pairs an existing Linear binding with its One project name.
type ProjectBinding struct {
	Name    string  `json:"name"`
	Binding Binding `json:"binding"`
}

func (s *Service) ListBindings(ctx context.Context) ([]ProjectBinding, error) {
	listing, err := s.groups.List(ctx)
	if err != nil {
		return nil, err
	}
	results := []ProjectBinding{}
	err = s.store.access(ctx, func(doc *bindingDocument) (bool, error) {
		add := func(kind, id, name string) {
			if binding, ok := doc.Bindings[(Scope{Kind: kind, ID: id}).Key()]; ok {
				results = append(results, ProjectBinding{Name: name, Binding: binding})
			}
		}
		for _, group := range listing.Groups {
			add("group", group.ID, group.Name)
		}
		for _, workspace := range listing.Workspaces {
			add("workspace", workspace.EntryID, workspace.Name)
		}
		return false, nil
	})
	return results, err
}
