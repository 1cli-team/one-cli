package workspace

import (
	"context"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	core "github.com/torchstellar-team/one-cli/packages/one-core/pkg/core/workspace"
)

// load reconciles old directory records and the former Git index into the
// workspace registry. Git remains the source of live checkout membership.
func (s *RegistryService) load(ctx context.Context) (core.Registry, error) {
	registry, err := s.repository.Load(ctx)
	if err != nil || s.git == nil {
		return registry, err
	}
	next := cloneWorkspaceRegistry(registry)
	if err := s.reconcile(ctx, &next); err != nil {
		return core.Registry{}, err
	}
	if reflect.DeepEqual(registry, next) {
		return registry, nil
	}
	err = s.repository.Update(ctx, func(current *core.Registry) error {
		if err := s.reconcile(ctx, current); err != nil {
			return err
		}
		next = cloneWorkspaceRegistry(*current)
		return nil
	})
	return next, err
}

type registryCandidate struct {
	entry        core.RegistryEntry
	originalRoot string
	originalID   string
	primary      bool
}

func (s *RegistryService) reconcile(ctx context.Context, registry *core.Registry) error {
	if s.git == nil {
		return nil
	}
	if !registry.GitRepositoriesImported && s.legacyGit != nil {
		entries, err := s.legacyGit()
		if err != nil {
			return err
		}
		for _, entry := range entries {
			known := false
			for index, current := range registry.Workspaces {
				if current.EntryID == entry.ID || current.GitCommonDir == entry.CommonDir {
					addRegistryAlias(&registry.Workspaces[index], core.RegistryAlias{EntryID: entry.ID})
					known = true
					break
				}
			}
			if !known {
				registry.Workspaces = append(registry.Workspaces, core.RegistryEntry{
					EntryID: entry.ID, Name: entry.Name, Root: entry.Path, GitCommonDir: entry.CommonDir,
					RegisteredAt: entry.RegisteredAt, LastSeenAt: entry.LastOpenedAt, LastSeenBy: "git-migration",
				})
			}
		}
		registry.GitRepositoriesImported = true
	}
	candidates := make([]registryCandidate, 0, len(registry.Workspaces))
	for _, entry := range registry.Workspaces {
		candidate := registryCandidate{entry: entry, originalRoot: entry.Root, originalID: entry.EntryID}
		info, err := s.git(ctx, entry.Root)
		if err != nil && entry.GitCommonDir != "" {
			info, err = s.git(ctx, entry.GitCommonDir)
		}
		if err == nil && len(info.Worktrees) > 0 {
			relative := entry.GitWorkspacePath
			if relative == "" {
				relative = "."
				longest := 0
				for _, tree := range info.Worktrees {
					rel, err := filepath.Rel(tree.Root, entry.Root)
					if err == nil && filepath.IsLocal(rel) && len(tree.Root) > longest {
						relative, longest = rel, len(tree.Root)
					}
				}
				if core.HasManifest(info.Worktrees[0].Root) {
					relative = "."
				}
			}
			candidate.entry.Root = filepath.Join(info.Worktrees[0].Root, relative)
			candidate.entry.GitCommonDir = info.CommonDir
			candidate.entry.GitWorkspacePath = relative
			candidate.primary = rootsReferToSamePath(entry.Root, candidate.entry.Root) || len(entry.Worktrees) > 0
			candidate.entry.Worktrees = make([]core.RegistryWorktree, 0, len(info.Worktrees))
			for _, tree := range info.Worktrees {
				candidate.entry.Worktrees = append(candidate.entry.Worktrees, core.RegistryWorktree{ID: tree.ID, Root: filepath.Join(tree.Root, relative)})
			}
			if candidate.entry.WorkspaceID == "" {
				inspection := inspectWorkspaceRoot(candidate.entry.Root)
				candidate.entry.WorkspaceID = inspection.id
				if inspection.name != "" {
					candidate.entry.Name = inspection.name
				}
			}
		}
		candidates = append(candidates, candidate)
	}
	// Prefer an existing main-worktree record as the logical workspace identity.
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].primary && !candidates[j].primary })
	entries := make([]core.RegistryEntry, 0, len(candidates))
	groups := make(map[string]int)
	for _, candidate := range candidates {
		entry := candidate.entry
		if entry.GitCommonDir == "" {
			entries = append(entries, entry)
			continue
		}
		index, exists := groups[entry.GitCommonDir]
		if !exists {
			if !candidate.primary && len(entry.Worktrees) > 0 {
				id, err := generateRegistryEntryID()
				if err != nil {
					return err
				}
				entry.EntryID = id
			}
			index = len(entries)
			groups[entry.GitCommonDir] = index
			entries = append(entries, entry)
		}
		parent := &entries[index]
		if candidate.entry.LastSeenAt.After(parent.LastSeenAt) {
			parent.LastSeenAt = candidate.entry.LastSeenAt
		}
		if parent.WorkspaceID == "" {
			parent.WorkspaceID = candidate.entry.WorkspaceID
		}
		for _, alias := range candidate.entry.Aliases {
			addRegistryAlias(parent, alias)
		}
		if candidate.originalID != parent.EntryID {
			alias := core.RegistryAlias{EntryID: candidate.originalID}
			longest := 0
			for _, tree := range parent.Worktrees {
				relative, err := filepath.Rel(tree.Root, candidate.originalRoot)
				if err == nil && filepath.IsLocal(relative) && len(tree.Root) > longest {
					alias.WorktreeID = tree.ID
					longest = len(tree.Root)
				}
			}
			addRegistryAlias(parent, alias)
		}
	}
	registry.Workspaces = entries
	return nil
}

func addRegistryAlias(entry *core.RegistryEntry, alias core.RegistryAlias) {
	if alias.EntryID == entry.EntryID {
		return
	}
	for _, existing := range entry.Aliases {
		if existing.EntryID == alias.EntryID {
			return
		}
	}
	entry.Aliases = append(entry.Aliases, alias)
}

func findRegistryEntry(registry core.Registry, requested string) (core.RegistryEntry, string, bool) {
	id, checkout, _ := strings.Cut(requested, "~")
	for _, entry := range registry.Workspaces {
		if entry.EntryID == id {
			return entry, checkout, true
		}
		for _, alias := range entry.Aliases {
			if alias.EntryID == id {
				if checkout == "" {
					checkout = alias.WorktreeID
				}
				return entry, checkout, true
			}
		}
	}
	return core.RegistryEntry{}, "", false
}

func (s *RegistryService) GitLocation(ctx context.Context, id string) (RegisteredWorkspace, error) {
	return s.location(ctx, id)
}

func (s *RegistryService) Relocate(ctx context.Context, id, root string) (RegisteredWorkspace, error) {
	canonical, err := canonicalRegistryRoot(root)
	if err != nil {
		return RegisteredWorkspace{}, err
	}
	if !core.HasManifest(canonical) {
		if s.git == nil {
			return RegisteredWorkspace{}, ErrRegistryEntryUnavailable
		}
		if _, err := s.git(ctx, canonical); err != nil {
			return RegisteredWorkspace{}, err
		}
	}
	registry, err := s.load(ctx)
	if err != nil {
		return RegisteredWorkspace{}, err
	}
	entry, _, ok := findRegistryEntry(registry, id)
	if !ok {
		return RegisteredWorkspace{}, ErrRegistryEntryNotFound
	}
	err = s.repository.Update(ctx, func(current *core.Registry) error {
		for index := range current.Workspaces {
			if current.Workspaces[index].EntryID != entry.EntryID {
				continue
			}
			current.Workspaces[index].Root = canonical
			current.Workspaces[index].GitCommonDir = ""
			current.Workspaces[index].GitWorkspacePath = ""
			current.Workspaces[index].Worktrees = nil
			return nil
		}
		return ErrRegistryEntryNotFound
	})
	if err != nil {
		return RegisteredWorkspace{}, err
	}
	return s.Observe(ctx, canonical, "relocate")
}
