package skills

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"regexp"
	"slices"
	"time"

	"github.com/torchstellar-team/one-cli/packages/one-core/internal/resources/bundled"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/fsutil"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

type HashFunc func(context.Context, string, map[string]Files) (map[string]string, error)

// Initializer installs embedded files or registers an unchanged existing copy.
// It seeds the standard project lock; all later updates belong to Skills CLI.
type Initializer struct {
	Hash    HashFunc
	Timeout time.Duration
}

type SkillInitialization struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

type InitResult struct {
	Schema    string                `json:"schema"`
	Root      string                `json:"root"`
	SourceRef string                `json:"sourceRef,omitempty"`
	DryRun    bool                  `json:"dryRun"`
	Skills    []SkillInitialization `json:"skills"`
	Changes   []string              `json:"changes"`
}

func (r *InitResult) RenderTTY(w io.Writer) {
	key := "skills.init_done"
	if r.DryRun {
		key = "skills.init_preview"
	}
	fmt.Fprintln(w, i18n.T(key))
	for _, skill := range r.Skills {
		fmt.Fprintf(w, "  %s: %s", skill.Name, i18n.T("skills.status."+skill.Status))
		if skill.Message != "" {
			fmt.Fprintf(w, " — %s", skill.Message)
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintln(w, i18n.T("skills.init_next"))
}

func officialSkillPath(name string) string {
	return "packages/agent-skills/skills/" + name + "/SKILL.md"
}

func (s Initializer) Init(ctx context.Context, root string, names []string, dryRun bool) (*InitResult, error) {
	timeout := s.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	unlock, err := fsutil.WorkspaceLock(ctx, root, "skills")
	if err != nil {
		return nil, err
	}
	defer unlock()
	return s.initLocked(ctx, root, names, dryRun)
}

var contentHash = regexp.MustCompile(`^[a-f0-9]{64}$`)

// The automatic installer already holds the same workspace lock.
func (s Initializer) initLocked(ctx context.Context, root string, names []string, dryRun bool) (*InitResult, error) {
	lockPath := filepath.Join(root, "skills-lock.json")
	if err := fsutil.SafeWritePath(root, lockPath); err != nil {
		return nil, err
	}
	plan := fsutil.NewFilePlan(root)
	lockRaw, err := plan.Read(lockPath)
	if err != nil {
		return nil, err
	}
	entries, err := decodeLock(lockRaw, lockPath)
	if err != nil {
		return nil, err
	}
	known, err := fs.ReadDir(bundled.SkillsFS, bundled.SkillsRoot)
	if err != nil {
		return nil, err
	}
	available := map[string]bool{}
	for _, entry := range known {
		if entry.IsDir() {
			available[entry.Name()] = true
		}
	}
	if len(names) == 0 {
		names = []string{"one-cli"}
		for name := range available {
			exists, err := installed(root, name)
			if err != nil {
				return nil, err
			}
			if _, locked := entries[name]; exists || locked {
				names = append(names, name)
			}
		}
	}
	names = slices.Clone(names)
	slices.Sort(names)
	names = slices.Compact(names)
	for _, name := range names {
		if !available[name] {
			return nil, i18n.Errorf("skills.unknown_official", name)
		}
	}
	result := &InitResult{Schema: "one-cli/init-skills/v1", Root: root, SourceRef: bundledSourceRef, DryRun: dryRun, Skills: []SkillInitialization{}, Changes: []string{}}
	files, originals := map[string]Files{}, map[string]Files{}
	matchesBundle := map[string]bool{}
	for _, name := range names {
		entry, locked := entries[name]
		exists, err := installed(root, name)
		if err != nil {
			return nil, err
		}
		item := SkillInitialization{Name: name}
		skip := func(reason string) {
			item.Status, item.Reason, item.Message = "skipped", reason, i18n.T("skills.skip."+reason)
		}
		switch {
		case locked && !sameSource(entry, ""):
			skip("source")
		case entry.SourceType == "github" && !exists:
			// Reinstalling a remote-managed copy from this CLI could replace a
			// deliberately chosen version. Let its existing manager restore it.
			skip("missing")
		case entry.SourceType == "github" && entry.SkillPath != "" && entry.ComputedHash != "":
			item.Status = "managed"
		case !exists:
			files[name], err = bundledFiles(name)
			if err != nil {
				return nil, err
			}
			item.Status = "would_install"
		default:
			current, err := snapshotSkill(root, name)
			if err == nil {
				err = checkSkillCopies(root, name)
			}
			if err != nil {
				skip("unsafe")
				item.Message = i18n.Tf("skills.skip_path", err)
				break
			}
			target, err := bundledFiles(name)
			if err != nil {
				return nil, err
			}
			matchesBundle[name] = sameFiles(current, target)
			if entry.ComputedHash == "" && !matchesBundle[name] {
				skip("untracked")
				break
			}
			files[name], originals[name] = current, current
			item.Status = "would_register"
		}
		result.Skills = append(result.Skills, item)
	}
	if len(files) == 0 {
		return result, nil
	}
	if s.Hash == nil {
		return nil, i18n.Errorf("skills.hash_unavailable")
	}
	hashes, err := s.Hash(ctx, root, files)
	if err != nil {
		return nil, err
	}
	replacements := map[string]json.RawMessage{}
	for index := range result.Skills {
		item := &result.Skills[index]
		name := item.Name
		if _, pending := files[name]; !pending {
			continue
		}
		if !contentHash.MatchString(hashes[name]) {
			return nil, i18n.Errorf("skills.hash_invalid", name)
		}
		if item.Status == "would_register" && !matchesBundle[name] && hashes[name] != entries[name].ComputedHash {
			item.Status, item.Reason, item.Message = "skipped", "modified", i18n.T("skills.skip.modified")
			continue
		}
		for path, raw := range files[name] {
			path = filepath.Join(".agents", "skills", name, filepath.FromSlash(path))
			if item.Status == "would_install" {
				if err := plan.Expect(path, nil); err != nil {
					return nil, err
				}
				if err := plan.Set(path, raw, 0o644); err != nil {
					return nil, err
				}
			} else if err := plan.Expect(path, raw); err != nil {
				return nil, err
			}
		}
		replacements[name], err = registration(lockRaw, name, hashes[name], entries[name])
		if err != nil {
			return nil, err
		}
	}
	if len(replacements) == 0 {
		return result, nil
	}
	merged, err := mergeLock(lockRaw, replacements)
	if err != nil {
		return nil, err
	}
	if err := plan.Set(lockPath, merged, 0o644); err != nil {
		return nil, err
	}
	for _, change := range plan.Changes() {
		result.Changes = append(result.Changes, change.Path)
	}
	if dryRun {
		return result, nil
	}
	for name := range replacements {
		if original, existed := originals[name]; existed {
			if err := checkSkillCopies(root, name); err != nil {
				return nil, err
			}
			current, err := snapshotSkill(root, name)
			if err != nil {
				return nil, err
			}
			if !sameFiles(current, original) {
				return nil, i18n.Errorf("skills.init_conflict", name)
			}
		} else if _, err := snapshotSkill(root, name); !errors.Is(err, fs.ErrNotExist) {
			return nil, i18n.Errorf("skills.init_conflict", name)
		}
	}
	if err := plan.Apply(ctx); err != nil {
		return nil, err
	}
	for index := range result.Skills {
		switch result.Skills[index].Status {
		case "would_install":
			result.Skills[index].Status = "installed"
		case "would_register":
			result.Skills[index].Status = "registered"
		}
	}
	return result, nil
}

// Seed only the fields understood by skills@1.7.0. Hash the installed bytes,
// not remote HEAD: a bundled version may legitimately lag behind master.
func registration(original []byte, name, hash string, entry lockEntry) (json.RawMessage, error) {
	fields := map[string]json.RawMessage{}
	if original != nil {
		var lock struct {
			Skills map[string]json.RawMessage `json:"skills"`
		}
		if err := json.Unmarshal(original, &lock); err != nil {
			return nil, err
		}
		if raw := lock.Skills[name]; len(raw) != 0 && string(raw) != "null" {
			if err := json.Unmarshal(raw, &fields); err != nil {
				return nil, err
			}
		}
	}
	ref, path := "master", officialSkillPath(name)
	if entry.SourceType == "github" {
		ref = entry.Ref
		if entry.SkillPath != "" {
			path = entry.SkillPath
		}
	} else {
		delete(fields, "sourceUrl")
	}
	delete(fields, "ref")
	raw, err := json.Marshal(lockEntry{Source: oneSource, SourceType: "github", Ref: ref, SkillPath: path, ComputedHash: hash})
	if err != nil {
		return nil, err
	}
	var managed map[string]json.RawMessage
	if err := json.Unmarshal(raw, &managed); err != nil {
		return nil, err
	}
	for key, value := range managed {
		fields[key] = value
	}
	return json.Marshal(fields)
}

// Keep unrelated skills and unknown fields in the standard upstream lock.
func mergeLock(original []byte, replacements map[string]json.RawMessage) ([]byte, error) {
	document := map[string]json.RawMessage{"version": json.RawMessage("1")}
	entries := map[string]json.RawMessage{}
	if original != nil {
		if err := json.Unmarshal(original, &document); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(document["skills"], &entries); err != nil {
			return nil, err
		}
	}
	for name, raw := range replacements {
		entries[name] = raw
	}
	raw, err := json.Marshal(entries)
	if err != nil {
		return nil, err
	}
	document["skills"] = raw
	raw, err = json.MarshalIndent(document, "", "  ")
	return append(raw, '\n'), err
}
