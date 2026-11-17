// Package workspacegroups organizes logical workspaces into business projects.
package workspacegroups

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"slices"
	"strings"
	"time"

	workspaceapp "github.com/torchstellar-team/one-cli/packages/one-core/pkg/application/workspace"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

var ErrNotFound = errors.New("workspace group not found")
var ErrInvalid = errors.New("invalid workspace group operation")

type Service struct{ registry *workspaceapp.RegistryService }

func NewService(registry *workspaceapp.RegistryService) *Service { return &Service{registry: registry} }

type Result struct {
	Schema                string                             `json:"schema"`
	Projects              []Project                          `json:"projects"`
	Groups                []Group                            `json:"groups"`
	Workspaces            []workspaceapp.RegisteredWorkspace `json:"workspaces"`
	UngroupedWorkspaceIDs []string                           `json:"ungroupedWorkspaceIds"`
}

// Project is a navigation entry backed by a workspace or a workspace group.
// Standalone projects are derived from the workspace registry on each read.
type Project struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Kind         string   `json:"kind"`
	WorkspaceIDs []string `json:"workspaceIds"`
}

type Detail struct {
	Schema     string                             `json:"schema"`
	Group      Group                              `json:"group"`
	Workspaces []workspaceapp.RegisteredWorkspace `json:"workspaces"`
}

type groupError struct {
	cause error
	key   string
	args  []any
}

func (e *groupError) Error() string { return i18n.Tf(e.key, e.args...) }
func (e *groupError) Unwrap() error { return e.cause }
func invalid(key string, args ...any) error {
	return &groupError{cause: ErrInvalid, key: key, args: args}
}

func groupIndex(groups []Group, reference string) (int, error) {
	for index, group := range groups {
		if group.ID == reference {
			return index, nil
		}
	}
	index := -1
	for candidate, group := range groups {
		if group.Name != reference {
			continue
		}
		if index >= 0 {
			return -1, invalid("groups.ambiguous", reference)
		}
		index = candidate
	}
	if index < 0 {
		return -1, &groupError{cause: ErrNotFound, key: "groups.not_found", args: []any{reference}}
	}
	return index, nil
}

func canonicalWorkspaceIDs(workspaces []workspaceapp.RegisteredWorkspace) map[string]string {
	result := make(map[string]string)
	for _, workspace := range workspaces {
		result[workspace.EntryID] = workspace.EntryID
		for _, alias := range workspace.Aliases {
			result[alias.EntryID] = workspace.EntryID
		}
	}
	return result
}

func normalizeMembers(groups []Group, canonical map[string]string) {
	for index := range groups {
		members := []string{}
		for _, id := range groups[index].WorkspaceIDs {
			if resolved := canonical[id]; resolved != "" {
				id = resolved
			}
			if !slices.Contains(members, id) {
				members = append(members, id)
			}
		}
		groups[index].WorkspaceIDs = members
	}
}

func (s *Service) List(ctx context.Context) (*Result, error) {
	workspaces, err := s.registry.List(ctx, "")
	if err != nil {
		return nil, err
	}
	path, err := storagePath()
	if err != nil {
		return nil, err
	}
	state, err := readDocument(path)
	if err != nil {
		return nil, err
	}
	normalizeMembers(state.Groups, canonicalWorkspaceIDs(workspaces.Workspaces))
	result := &Result{
		Schema: "one-cli/workspace-groups/v1", Projects: []Project{}, Groups: state.Groups,
		Workspaces: workspaces.Workspaces, UngroupedWorkspaceIDs: []string{},
	}
	used := make(map[string]bool)
	for _, group := range result.Groups {
		result.Projects = append(result.Projects, Project{
			ID: group.ID, Name: group.Name, Kind: "group", WorkspaceIDs: slices.Clone(group.WorkspaceIDs),
		})
		for _, id := range group.WorkspaceIDs {
			used[id] = true
		}
	}
	for _, workspace := range result.Workspaces {
		if !used[workspace.EntryID] {
			result.UngroupedWorkspaceIDs = append(result.UngroupedWorkspaceIDs, workspace.EntryID)
			result.Projects = append(result.Projects, Project{
				ID: workspace.EntryID, Name: workspace.Name, Kind: "workspace", WorkspaceIDs: []string{workspace.EntryID},
			})
		}
	}
	return result, nil
}

func (s *Service) Get(ctx context.Context, reference string) (*Detail, error) {
	result, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	index, err := groupIndex(result.Groups, reference)
	if err != nil {
		return nil, err
	}
	group := result.Groups[index]
	detail := &Detail{Schema: "one-cli/workspace-group/v1", Group: group, Workspaces: []workspaceapp.RegisteredWorkspace{}}
	for _, id := range group.WorkspaceIDs {
		for _, workspace := range result.Workspaces {
			if workspace.EntryID == id {
				detail.Workspaces = append(detail.Workspaces, workspace)
				break
			}
		}
	}
	return detail, nil
}

func (s *Service) Create(ctx context.Context, name string) (*Group, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, invalid("groups.name_required")
	}
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	group := Group{ID: "wsg_" + hex.EncodeToString(bytes[:]), Name: name, WorkspaceIDs: []string{}, CreatedAt: now, UpdatedAt: now}
	err := updateDocument(ctx, func(state *document) error { state.Groups = append(state.Groups, group); return nil })
	return &group, err
}

func (s *Service) change(ctx context.Context, reference string, change func(*Group, map[string]string) error) (*Group, error) {
	workspaces, err := s.registry.List(ctx, "")
	if err != nil {
		return nil, err
	}
	canonical := canonicalWorkspaceIDs(workspaces.Workspaces)
	var result Group
	err = updateDocument(ctx, func(state *document) error {
		normalizeMembers(state.Groups, canonical)
		index, err := groupIndex(state.Groups, reference)
		if err != nil {
			return err
		}
		group := &state.Groups[index]
		if err := change(group, canonical); err != nil {
			return err
		}
		group.UpdatedAt = time.Now().UTC()
		result = *group
		return nil
	})
	return &result, err
}

func (s *Service) Rename(ctx context.Context, reference, name string) (*Group, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, invalid("groups.name_required")
	}
	return s.change(ctx, reference, func(group *Group, _ map[string]string) error { group.Name = name; return nil })
}

func (s *Service) Delete(ctx context.Context, reference string) error {
	return updateDocument(ctx, func(state *document) error {
		index, err := groupIndex(state.Groups, reference)
		if err != nil {
			return err
		}
		state.Groups = append(state.Groups[:index], state.Groups[index+1:]...)
		return nil
	})
}

func (s *Service) Add(ctx context.Context, reference string, ids []string) (*Group, error) {
	if len(ids) == 0 {
		return nil, invalid("groups.members_required")
	}
	return s.change(ctx, reference, func(group *Group, canonical map[string]string) error {
		for _, id := range ids {
			logical, _, _ := strings.Cut(id, "~")
			member := canonical[logical]
			if member == "" {
				return invalid("groups.workspace_not_found", id)
			}
			if !slices.Contains(group.WorkspaceIDs, member) {
				group.WorkspaceIDs = append(group.WorkspaceIDs, member)
			}
		}
		return nil
	})
}

func (s *Service) Remove(ctx context.Context, reference string, ids []string) (*Group, error) {
	if len(ids) == 0 {
		return nil, invalid("groups.members_required")
	}
	return s.change(ctx, reference, func(group *Group, canonical map[string]string) error {
		for _, id := range ids {
			logical, _, _ := strings.Cut(id, "~")
			if resolved := canonical[logical]; resolved != "" {
				logical = resolved
			}
			group.WorkspaceIDs = slices.DeleteFunc(group.WorkspaceIDs, func(member string) bool { return member == logical })
		}
		return nil
	})
}

// Move places a member immediately before another member. An empty before ID
// moves it to the end. Replaying a move keeps the same relative placement.
func (s *Service) Move(ctx context.Context, reference, id, before string) (*Group, error) {
	return s.change(ctx, reference, func(group *Group, canonical map[string]string) error {
		id, _, _ = strings.Cut(id, "~")
		before, _, _ = strings.Cut(before, "~")
		if resolved := canonical[id]; resolved != "" {
			id = resolved
		}
		if resolved := canonical[before]; resolved != "" {
			before = resolved
		}
		if !slices.Contains(group.WorkspaceIDs, id) || (before != "" && !slices.Contains(group.WorkspaceIDs, before)) {
			return invalid("groups.member_not_found")
		}
		if id == before {
			return nil
		}
		members := slices.DeleteFunc(group.WorkspaceIDs, func(member string) bool { return member == id })
		index := len(members)
		if before != "" {
			index = slices.Index(members, before)
		}
		group.WorkspaceIDs = slices.Insert(members, index, id)
		return nil
	})
}
