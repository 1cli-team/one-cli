// Package skill is the install pipeline for agentskills.io-format
// skills. It pairs pkg/agentskills (the path table + scope semantics)
// with a canonical on-disk store at ~/.one/skills-store/, where
// physical skill directories live; the per-agent paths are symlinks
// (or, on Windows, copies) pointing at the store.
//
// Architecture:
//
//	source (bundled / GitHub / local)
//	      │ Fetch
//	      ▼
//	~/.one/skills-store/<source>/<skill-name>/    canonical
//	      │ link (symlink or copy)
//	      ▼
//	<scope>/<agent-path>/<skill-name>             per-agent
//
// The manifest at ~/.one/skills-store/.manifest.json tracks every
// (skill, source, scope, agent) install so list / remove / update can
// find what's been put where without re-scanning every supported agent.
package skill

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/torchstellar-team/one-cli/packages/cli/pkg/agentskills"
)

// StoreDir returns the absolute path to the canonical skill store.
// Always under the user's home dir; we don't honour XDG_DATA_HOME
// because this is one-cli-internal state, not user-facing config.
func StoreDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".one", "skills-store"), nil
}

// SkillStorePath returns the canonical path inside the store for a
// skill with the given source id and skill name. The directory is
// where Fetch writes the physical copy; subsequent installs link from
// there to each agent's path.
func SkillStorePath(sourceID, skillName string) (string, error) {
	root, err := StoreDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, sourceID, skillName), nil
}

// EnsureStoreDir creates the store root if missing. Idempotent.
func EnsureStoreDir() error {
	root, err := StoreDir()
	if err != nil {
		return err
	}
	return os.MkdirAll(root, 0o755)
}

// Manifest tracks every skill one-cli has installed across the user's
// machine. It lives at <StoreDir>/.manifest.json so a single read
// answers `one skill list` without scanning every agent path.
type Manifest struct {
	Schema  string          `json:"schema"`
	Entries []ManifestEntry `json:"entries"`
}

// ManifestEntry is one installed (skill, agent, scope) tuple. A skill
// installed to N agents produces N entries — easier to remove a single
// install than to delete and re-emit a "skill installed to N" record.
type ManifestEntry struct {
	SkillName   string             `json:"skill_name"`
	SourceID    string             `json:"source_id"`    // matches store path component
	SourceLabel string             `json:"source_label"` // human-readable, e.g. "vercel-labs/agent-skills"
	AgentID     string             `json:"agent_id"`     // matches agentskills.Agent.ID
	Scope       agentskills.Scope  `json:"scope"`
	Method      agentskills.Method `json:"method"`
	InstalledAt string             `json:"installed_at"` // RFC 3339
	Version     string             `json:"version,omitempty"`
}

const manifestSchema = "one-cli/skill-manifest/v1"

// LoadManifest reads the manifest from disk. Returns a fresh empty
// manifest when the file is absent — first-run safe.
func LoadManifest() (*Manifest, error) {
	root, err := StoreDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(root, ".manifest.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &Manifest{Schema: manifestSchema, Entries: []ManifestEntry{}}, nil
		}
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	if m.Schema == "" {
		m.Schema = manifestSchema
	}
	if m.Entries == nil {
		m.Entries = []ManifestEntry{}
	}
	return &m, nil
}

// SaveManifest writes the manifest atomically (temp file + rename).
func SaveManifest(m *Manifest) error {
	if m == nil {
		return errors.New("nil manifest")
	}
	if m.Schema == "" {
		m.Schema = manifestSchema
	}
	if err := EnsureStoreDir(); err != nil {
		return err
	}
	root, err := StoreDir()
	if err != nil {
		return err
	}
	path := filepath.Join(root, ".manifest.json")
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Append adds an entry, replacing any existing entry with the same
// (SkillName, AgentID, Scope) tuple — re-installs are upsert.
func (m *Manifest) Append(e ManifestEntry) {
	for i, existing := range m.Entries {
		if existing.SkillName == e.SkillName && existing.AgentID == e.AgentID && existing.Scope == e.Scope {
			m.Entries[i] = e
			return
		}
	}
	m.Entries = append(m.Entries, e)
}

// Remove drops every entry matching the given predicate. Returns the
// number of entries removed.
func (m *Manifest) Remove(match func(ManifestEntry) bool) int {
	keep := m.Entries[:0]
	removed := 0
	for _, e := range m.Entries {
		if match(e) {
			removed++
			continue
		}
		keep = append(keep, e)
	}
	m.Entries = keep
	return removed
}
