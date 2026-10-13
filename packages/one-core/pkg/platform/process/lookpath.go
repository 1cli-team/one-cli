package process

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// LookPathIn resolves tools using the child's PATH without changing process globals.
func LookPathIn(name, dir string, env []string) (string, error) {
	if strings.ContainsAny(name, `/\`) {
		if !filepath.IsAbs(name) {
			name = filepath.Join(dir, name)
		}
		return exec.LookPath(name)
	}
	path := ""
	for _, entry := range env {
		key, value, ok := strings.Cut(entry, "=")
		if ok && (key == "PATH" || runtime.GOOS == "windows" && strings.EqualFold(key, "PATH")) {
			path = value
		}
	}
	for _, part := range filepath.SplitList(path) {
		if part == "" {
			continue
		}
		if !filepath.IsAbs(part) {
			part = filepath.Join(dir, part)
		}
		candidate, err := exec.LookPath(filepath.Join(part, name))
		if err == nil {
			return candidate, nil
		}
	}
	return "", &exec.Error{Name: name, Err: exec.ErrNotFound}
}
