package issuetracking

import (
	"context"
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
	status, err := s.Backend.Status(ctx)
	if err != nil {
		return Binding{}, err
	}
	var connection *Connection
	for _, candidate := range status.Connections {
		if candidate.ID == input.ConnectionID {
			copy := candidate
			connection = &copy
			break
		}
	}
	if connection == nil {
		return Binding{}, Fail("connection")
	}
	project, err := s.Backend.Project(ctx, connection.ID, input.Project.ID)
	if err != nil {
		return Binding{}, err
	}
	found := false
	for _, team := range project.Teams {
		if team.ID == input.TeamID {
			found = true
		}
	}
	if !found {
		return Binding{}, Fail("team")
	}
	binding := Binding{Scope: scope, ConnectionID: connection.ID, OrganizationID: connection.OrganizationID, Project: project, TeamID: input.TeamID}
	if err := s.store.put(ctx, scope, &binding); err != nil {
		return Binding{}, err
	}
	return binding, nil
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
