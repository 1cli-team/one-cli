package skills

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/one-core/internal/resources/bundled"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/fsutil"
	"github.com/torchstellar-team/one-cli/packages/one-core/pkg/platform/i18n"
)

// Files is an immutable snapshot, with slash-separated paths relative to a skill.
type Files map[string][]byte

func bundledFiles(name string) (Files, error) {
	files := Files{}
	skill, err := fs.Sub(bundled.SkillsFS, bundled.SkillsRoot+"/"+name)
	if err != nil {
		return nil, err
	}
	err = fs.WalkDir(skill, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		files[path], err = fs.ReadFile(skill, path)
		return err
	})
	return files, err
}

func snapshotSkill(root, name string) (Files, error) {
	directory := filepath.Join(root, ".agents", "skills", name)
	if err := fsutil.SafeWritePath(root, directory); err != nil {
		return nil, err
	}
	files := Files{}
	err := filepath.WalkDir(directory, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			// Legacy hashes exclude these directories. Keep registration
			// conservative for copies containing dependencies or Git data.
			if entry.Name() == ".git" || entry.Name() == "node_modules" {
				return i18n.Errorf("skills.untracked_path", path)
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return i18n.Errorf("skills.untracked_path", path)
		}
		relative, err := filepath.Rel(directory, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if strings.Contains(relative, "\\") {
			return i18n.Errorf("skills.untracked_path", path)
		}
		files[relative], err = os.ReadFile(path)
		return err
	})
	return files, err
}

func checkSkillCopies(root, name string) error {
	canonical, err := filepath.EvalSymlinks(filepath.Join(root, ".agents", "skills", name))
	if err != nil {
		return err
	}
	for _, directory := range []string{".claude/skills", ".cursor/skills", ".codex/skills"} {
		path := filepath.Join(root, directory, name)
		other, err := filepath.EvalSymlinks(path)
		if errors.Is(err, fs.ErrNotExist) {
			// A dangling link is still a separately managed placement.
			if _, statErr := os.Lstat(path); errors.Is(statErr, fs.ErrNotExist) {
				continue
			}
		}
		if err != nil || other != canonical {
			return i18n.Errorf("skills.multiple_copies", path)
		}
	}
	return nil
}

func sameFiles(a, b Files) bool {
	if len(a) != len(b) {
		return false
	}
	for path, raw := range a {
		other, ok := b[path]
		if !ok || !bytes.Equal(raw, other) {
			return false
		}
	}
	return true
}
