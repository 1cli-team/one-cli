package skills

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/fsutil"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	platformprocess "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/process"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/resources/bundled"
)

type RunFunc func(context.Context, string, []string, io.Reader, io.Writer, io.Writer) error

// Installer adds missing skills only. Upstream owns skills-lock.json; One does
// not implement a second skills registry or rewrite user-managed installations.
type Installer struct {
	Run      RunFunc
	Log      io.Writer
	Progress func(context.Context, string)
	Timeout  time.Duration
}

func (s Installer) Install(ctx context.Context, root string, selections []Selection) []string {
	timeout := s.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	unlock, err := fsutil.WorkspaceLock(ctx, root, "skills")
	if err != nil {
		return []string{i18n.Tf("skills.install_warning", root, err, "one skills --help")}
	}
	defer unlock()
	lock, err := readLock(root)
	if err != nil {
		return []string{i18n.Tf("skills.install_warning", root, err, "one skills list")}
	}
	log := s.Log
	if log == nil {
		log = io.Discard
	}
	seen := map[string]bool{}
	var warnings []string
	for _, selection := range selections {
		var missing []string
		for _, name := range selection.Names {
			if seen[name] {
				continue
			}
			seen[name] = true
			entry, locked := lock[name]
			if locked && !sameSource(entry, selection.Source) {
				warnings = append(warnings, i18n.Tf("skills.source_conflict", name, entry.Source))
				continue
			}
			exists, err := installed(root, name)
			if err != nil {
				warnings = append(warnings, i18n.Tf("skills.install_warning", name, err, "one skills list"))
				continue
			}
			retryRegistration := false
			if exists && !locked && selection.Source == "" && bundledSourceRef == "" {
				// A failed development registration can leave the copied files
				// without provenance. Retry only unchanged bundled copies so
				// unregistered team instructions remain untouched.
				retryRegistration, err = matchesBundledSkill(root, name)
				if err != nil {
					warnings = append(warnings, i18n.Tf("skills.install_warning", name, err, "one skills list"))
					continue
				}
			}
			if !exists || retryRegistration {
				missing = append(missing, name)
			}
		}
		if len(missing) == 0 {
			continue
		}
		// Prevent upstream writes through a workspace-owned symlink. Existing
		// individual skills were already skipped and remain untouched.
		if err := fsutil.SafeWritePath(root, filepath.Join(root, ".agents", "skills")); err != nil {
			warnings = append(warnings, i18n.Tf("skills.install_warning", strings.Join(missing, ", "), err, "one skills list"))
			continue
		}
		if err := fsutil.SafeWritePath(root, filepath.Join(root, "skills-lock.json")); err != nil {
			warnings = append(warnings, i18n.Tf("skills.install_warning", strings.Join(missing, ", "), err, "one skills list"))
			continue
		}
		source := selection.Source
		if source == "" {
			source = bundledSource()
			if bundledSourceRef == "" {
				if err := writeBundled(ctx, root, missing); err != nil {
					warnings = append(warnings, i18n.Tf("skills.install_warning", strings.Join(missing, ", "), err, "one skills --help"))
					continue
				}
			}
		}
		args := []string{"add", source, "--skill"}
		args = append(args, missing...)
		args = append(args, "--agent", "universal", "--copy", "--yes")
		recovery := recoveryCommand(source, missing)
		message := i18n.Tf("skills.installing", strings.Join(missing, ", "))
		if s.Progress != nil {
			s.Progress(ctx, message)
		} else {
			fmt.Fprintln(log, message)
		}
		diagnostics := &diagnosticLog{}
		if ctx.Err() != nil {
			err = ctx.Err()
		} else if s.Run == nil {
			err = errors.New(i18n.T("skills.runner_required"))
		} else {
			err = s.Run(ctx, root, args, nil, diagnostics, diagnostics)
		}
		if ctx.Err() != nil {
			err = ctx.Err()
		} else if err != nil && diagnostics.text() != "" {
			err = i18n.Errorf("skills.command_failed", platformprocess.ExitCode(err), err, diagnostics.text())
		}
		if err == nil {
			for _, name := range missing {
				if exists, checkErr := installed(root, name); checkErr != nil {
					err = checkErr
					break
				} else if !exists {
					err = i18n.Errorf("skills.install_missing", name)
					break
				}
			}
		}
		if err != nil {
			warnings = append(warnings, i18n.Tf("skills.install_warning", strings.Join(missing, ", "), err, recovery))
		}
	}
	return warnings
}

type lockEntry struct {
	Source     string `json:"source"`
	SourceType string `json:"sourceType"`
}

func readLock(root string) (map[string]lockEntry, error) {
	path := filepath.Join(root, "skills-lock.json")
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]lockEntry{}, nil
	}
	if err != nil {
		return nil, err
	}
	var lock struct {
		Version int                  `json:"version"`
		Skills  map[string]lockEntry `json:"skills"`
	}
	if err := json.Unmarshal(raw, &lock); err != nil {
		return nil, i18n.Errorf("skills.lock_invalid", path, err)
	}
	if lock.Version != 1 || lock.Skills == nil {
		return nil, i18n.Errorf("skills.lock_unsupported", path)
	}
	return lock.Skills, nil
}

func sameSource(entry lockEntry, source string) bool {
	if source == "" {
		if entry.SourceType == "github" {
			return entry.Source == oneSource
		}
		// Preserve old local-source locks and team edits when adding projects.
		// Migration to a release source is an explicit upstream skills operation.
		return entry.SourceType == "local" && (strings.HasPrefix(filepath.ToSlash(entry.Source), "./.one/skill-sources/") || entry.Source == "./packages/agent-skills" || entry.Source == "./.agents/skills")
	}
	return entry.SourceType == "github" && entry.Source == sourceRepo(source)
}

func installed(root, name string) (bool, error) {
	for _, directory := range []string{".agents/skills", ".claude/skills", ".cursor/skills", ".codex/skills"} {
		_, err := os.Lstat(filepath.Join(root, directory, name))
		if err == nil {
			return true, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return false, err
		}
	}
	return false, nil
}

func matchesBundledSkill(root, name string) (bool, error) {
	skill, err := fs.Sub(bundled.SkillsFS, bundled.SkillsRoot+"/"+name)
	if err != nil {
		return false, err
	}
	matches := true
	err = fs.WalkDir(skill, ".", func(relative string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		original, err := fs.ReadFile(skill, relative)
		if err != nil {
			return err
		}
		current, err := os.ReadFile(filepath.Join(root, ".agents", "skills", name, filepath.FromSlash(relative)))
		if errors.Is(err, fs.ErrNotExist) {
			matches = false
			return fs.SkipAll
		}
		if err != nil {
			return err
		}
		if !bytes.Equal(current, original) {
			matches = false
			return fs.SkipAll
		}
		return nil
	})
	return matches, err
}

// Development builds write only selected skills into their final project paths.
// Upstream registers these files as local sources without copying onto themselves.
// Releases install from their published commit instead. Neither needs a second
// source tree under .one or .agents/skill-sources.
func writeBundled(ctx context.Context, root string, names []string) error {
	assets, err := fs.Sub(bundled.SkillsFS, bundled.SkillsRoot)
	if err != nil {
		return err
	}
	plan := fsutil.NewFilePlan(root)
	for _, name := range names {
		skill, err := fs.Sub(assets, name)
		if err != nil {
			return err
		}
		if err := fs.WalkDir(skill, ".", func(relative string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}
			raw, err := fs.ReadFile(skill, relative)
			if err != nil {
				return err
			}
			path := ".agents/skills/" + name + "/" + relative
			before, err := plan.Read(path)
			if err != nil {
				return err
			}
			if before != nil && !bytes.Equal(before, raw) {
				return i18n.Errorf("skills.source_modified", path)
			}
			return plan.Set(path, raw, 0o644)
		}); err != nil {
			return err
		}
	}
	return plan.Apply(ctx)
}

func recoveryCommand(source string, names []string) string {
	return "one skills add " + source + " --skill " + strings.Join(names, " ") + " --agent universal --copy"
}

// Capture automatic-install output without rendering the upstream UI. Retain a
// bounded failure reason for result warnings. Manual skills commands use Runner
// directly and still forward both streams unchanged.
type diagnosticLog struct {
	mu   sync.Mutex
	tail []byte
}

func (d *diagnosticLog) Write(raw []byte) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.tail = append(d.tail, raw...)
	if len(d.tail) > 4096 {
		d.tail = append([]byte(nil), d.tail[len(d.tail)-4096:]...)
	}
	return len(raw), nil
}

var ansiControl = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)

func (d *diagnosticLog) text() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return strings.TrimSpace(ansiControl.ReplaceAllString(string(d.tail), ""))
}
