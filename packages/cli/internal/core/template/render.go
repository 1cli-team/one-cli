package template

import (
	"context"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	cliErrors "github.com/torchstellar-team/one-cli/packages/cli/internal/platform/errors"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/fsutil"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/i18n"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/platform/templatefiles"
	"github.com/torchstellar-team/one-cli/packages/cli/internal/resources/bundled"
)

// Variables contains only the two supported starter substitutions.
type Variables map[string]string

var (
	camelToWordsRE = regexp.MustCompile(`([a-z0-9])([A-Z])`)
	nonAlphaNumRE  = regexp.MustCompile(`[^a-zA-Z0-9]+`)
)

// CommonVariables keeps the existing project-name normalization.
func CommonVariables(projectName, _ string) Variables {
	words := nonAlphaNumRE.Split(
		camelToWordsRE.ReplaceAllString(strings.TrimSpace(projectName), "$1 $2"), -1)
	var nonempty []string
	for _, word := range words {
		if word != "" {
			nonempty = append(nonempty, strings.ToLower(word))
		}
	}
	return Variables{"projectName": projectName, "projectNameKebabCase": strings.Join(nonempty, "-")}
}

const LocalTemplatePrefix = "local:"

// Render prepares all template files before publishing any of them. Creation
// holds the workspace lock and remains responsible for workspace registration.
func Render(templateID, targetDir string, vars Variables) error {
	if err := validatePaths([]string{templateID}); err != nil {
		return err
	}
	source, err := fs.Sub(bundled.TemplatesFS, path.Join(bundled.TemplatesRoot, templateID))
	if err != nil {
		return err
	}
	if _, err := fs.Stat(source, "."); err != nil {
		return cliErrors.New(cliErrors.TEMPLATE_NOT_FOUND, i18n.Tf("template.local_missing", templateID, templateID))
	}
	files, err := prepare(source, vars)
	if err != nil {
		return i18n.Errorf("template.prepare_failed", templateID, err)
	}
	return publish(targetDir, files)
}

func prepare(source fs.FS, vars Variables) (map[string][]byte, error) {
	spec, err := readSpec(source)
	if err != nil {
		return nil, err
	}
	files := map[string][]byte{}
	err = fs.WalkDir(source, ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if name == "." {
			return nil
		}
		logical := templatefiles.LogicalPath(name)
		if templatefiles.Excluded(logical) || spec.excluded(logical) {
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			return i18n.Errorf("template.spec_path", name)
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return i18n.Errorf("template.spec_path", name)
		}
		if logical == "template.json" {
			return nil
		}
		if err := validatePaths([]string{logical}); err != nil {
			return err
		}
		if strings.HasSuffix(logical, ".hbs") {
			return i18n.Errorf("template.legacy_file", logical)
		}
		if _, exists := files[logical]; exists {
			return i18n.Errorf("template.file_collision", logical)
		}
		raw, err := fs.ReadFile(source, name)
		if err != nil {
			return err
		}
		files[logical] = append([]byte{}, raw...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := applyText(files, spec.Text, vars); err != nil {
		return nil, err
	}
	if spec.Go != nil {
		if err := rewriteGo(files, *spec.Go, vars); err != nil {
			return nil, err
		}
	}
	if spec.Node != nil {
		if err := rewriteNode(files, *spec.Node, vars); err != nil {
			return nil, err
		}
	}
	return files, nil
}

func sortedFiles(files map[string][]byte) []string {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func publish(target string, files map[string][]byte) (err error) {
	target, err = filepath.Abs(target)
	if err != nil {
		return err
	}
	if info, statErr := os.Lstat(target); statErr == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return i18n.Errorf("template.spec_path", target)
		}
		entries, readErr := os.ReadDir(target)
		if readErr != nil {
			return readErr
		}
		if len(entries) > 0 {
			return i18n.Errorf("creation.directory_exists", target)
		}
	} else if !os.IsNotExist(statErr) {
		return statErr
	}
	plan := fsutil.NewFilePlan(target)
	// Record only missing directories, removing them on failure only if empty.
	// Never recursively remove files that another writer might have created.
	missing := map[string]bool{}
	for _, name := range sortedFiles(files) {
		dest := filepath.Join(target, filepath.FromSlash(name))
		if err := fsutil.SafeWritePath(target, dest); err != nil {
			return err
		}
		for dir := filepath.Dir(dest); ; dir = filepath.Dir(dir) {
			if _, statErr := os.Lstat(dir); os.IsNotExist(statErr) {
				missing[dir] = true
			}
			if dir == target {
				break
			}
		}
		mode := os.FileMode(0o644)
		if strings.HasSuffix(name, ".sh") {
			mode = 0o755
		}
		if err := plan.Set(name, files[name], mode); err != nil {
			return err
		}
	}
	defer func() {
		if err == nil {
			return
		}
		dirs := make([]string, 0, len(missing))
		for dir := range missing {
			dirs = append(dirs, dir)
		}
		sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) })
		for _, dir := range dirs {
			_ = os.Remove(dir)
		}
	}()
	if len(files) == 0 {
		return os.MkdirAll(target, 0o755)
	}
	return plan.Apply(context.Background())
}
