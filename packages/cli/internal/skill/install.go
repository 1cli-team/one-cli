package skill

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/bundled"
	"github.com/torchstellar-team/one-cli/packages/cli/pkg/agentskills"
)

// bundledSourceID is the canonical name used as the directory under
// ~/.one/skills-store/. Stable across releases — changing it would
// orphan existing installs.
const bundledSourceID = "one-bundled"

// InstallBundled materialises every bundled skill into the canonical
// store and links each one into the supplied agents' skills
// directories. Always uses MethodSymlink, falling back to MethodCopy
// transparently when the OS rejects symlinks (Windows).
//
// External skill management — installing skills from GitHub /
// arbitrary git URLs / etc. — is out of scope. Use vercel-labs/skills
// (`npx skills add <source>`) for that workflow; the formats are
// compatible.
func InstallBundled(agents []agentskills.Agent, scope agentskills.Scope, workspaceRoot string) (*BundledInstallResult, error) {
	if len(agents) == 0 {
		return nil, fmt.Errorf("install: no target agents")
	}
	if scope == agentskills.ScopeProject && workspaceRoot == "" {
		return nil, fmt.Errorf("install: project scope requires workspaceRoot")
	}
	if err := EnsureStoreDir(); err != nil {
		return nil, err
	}

	skills, err := materialiseBundled()
	if err != nil {
		return nil, err
	}
	if len(skills) == 0 {
		return nil, fmt.Errorf("bundled skills tree is empty (build error?)")
	}

	manifest, err := LoadManifest()
	if err != nil {
		return nil, err
	}

	res := &BundledInstallResult{Skills: append([]string{}, skills...)}
	for _, skillName := range skills {
		storePath, err := SkillStorePath(bundledSourceID, skillName)
		if err != nil {
			return res, err
		}
		for _, agent := range agents {
			dest := agentskills.DestinationPath(agent, scope, workspaceRoot)
			if dest == "" {
				return res, fmt.Errorf("agent %q: no path for scope %q", agent.ID, scope)
			}
			finalMethod, err := linkOrCopy(storePath, filepath.Join(dest, skillName), agentskills.MethodSymlink)
			if err != nil {
				return res, fmt.Errorf("install %s → %s: %w", skillName, agent.ID, err)
			}
			entry := ManifestEntry{
				SkillName:   skillName,
				SourceID:    bundledSourceID,
				SourceLabel: "one-cli bundled",
				AgentID:     agent.ID,
				Scope:       scope,
				Method:      finalMethod,
				InstalledAt: time.Now().UTC().Format(time.RFC3339),
			}
			manifest.Append(entry)
			res.AgentEntries = append(res.AgentEntries, entry)
		}
	}

	// Garbage-collect manifest entries whose skill is no longer in the
	// bundled set (e.g. the v0.3.0 → v0.4.0 consolidation deletes 5
	// old skills). The on-disk symlinks at agent paths get cleaned up
	// in the same pass.
	pruneRemoved(manifest, skills, workspaceRoot)

	if err := SaveManifest(manifest); err != nil {
		return res, fmt.Errorf("save manifest: %w", err)
	}
	return res, nil
}

// BundledInstallResult is the report from InstallBundled — the list
// of skill names installed and one ManifestEntry per (skill × agent).
type BundledInstallResult struct {
	Skills       []string
	AgentEntries []ManifestEntry
}

// materialiseBundled walks bundled.SkillsFS and copies every
// top-level entry into ~/.one/skills-store/one-bundled/<name>/.
// Returns the names in registration order.
func materialiseBundled() ([]string, error) {
	root := bundled.SkillsRoot
	entries, err := fs.ReadDir(bundled.SkillsFS, root)
	if err != nil {
		return nil, fmt.Errorf("bundled skills fs: %w", err)
	}
	out := []string{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		dst, err := SkillStorePath(bundledSourceID, name)
		if err != nil {
			return nil, err
		}
		if err := materialiseEmbedDir(bundled.SkillsFS, filepath.ToSlash(filepath.Join(root, name)), dst); err != nil {
			return nil, fmt.Errorf("materialise %q: %w", name, err)
		}
		out = append(out, name)
	}
	return out, nil
}

// materialiseEmbedDir copies the contents of an fs.FS subtree into a
// real directory, removing any existing tree first.
func materialiseEmbedDir(srcFS fs.FS, srcRoot, dst string) error {
	if err := removeExisting(dst); err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	return fs.WalkDir(srcFS, srcRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(p, srcRoot)
		rel = strings.TrimPrefix(rel, "/")
		target := filepath.Join(dst, filepath.FromSlash(rel))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		raw, err := fs.ReadFile(srcFS, p)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, raw, 0o644)
	})
}

// pruneRemoved drops manifest entries (and their on-disk symlinks)
// for skills that used to be bundled but no longer are. Idempotent:
// safe to call when nothing's changed.
func pruneRemoved(manifest *Manifest, current []string, workspaceRoot string) {
	live := map[string]bool{}
	for _, name := range current {
		live[name] = true
	}
	manifest.Remove(func(e ManifestEntry) bool {
		if e.SourceID != bundledSourceID || live[e.SkillName] {
			return false
		}
		if agent, ok := agentskills.GetByID(e.AgentID); ok {
			dest := agentskills.DestinationPath(agent, e.Scope, workspaceRoot)
			if dest != "" {
				_ = removeExisting(filepath.Join(dest, e.SkillName))
			}
		}
		// Also drop the canonical store copy.
		if path, err := SkillStorePath(e.SourceID, e.SkillName); err == nil {
			_ = removeExisting(path)
		}
		return true
	})
}
