package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/format"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	if len(os.Args) != 2 {
		fatalf("usage: repo-checks <node-workspace|frontmatter|gofmt>")
	}
	root, err := findRepoRoot()
	if err != nil {
		fatalf("%v", err)
	}
	switch os.Args[1] {
	case "node-workspace":
		err = checkNodeWorkspace(root)
	case "frontmatter":
		err = checkFrontmatter(root)
	case "gofmt":
		err = checkGoFormat(root)
	default:
		fatalf("unknown check %q", os.Args[1])
	}
	if err != nil {
		fatalf("%v", err)
	}
}

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.work")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("repository root not found (go.work is missing)")
		}
		dir = parent
	}
}

func checkNodeWorkspace(root string) error {
	var problems []string
	locks, err := filepath.Glob(filepath.Join(root, "apps", "*", "pnpm-lock.yaml"))
	if err != nil {
		return err
	}
	problems = append(problems, locks...)

	manifests, err := filepath.Glob(filepath.Join(root, "apps", "*", "package.json"))
	if err != nil {
		return err
	}
	for _, path := range manifests {
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var manifest map[string]json.RawMessage
		if err := json.Unmarshal(raw, &manifest); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		if _, exists := manifest["packageManager"]; exists {
			problems = append(problems, path)
		}
	}
	if len(problems) == 0 {
		return nil
	}
	sort.Strings(problems)
	return fmt.Errorf("Node workspace metadata must be owned by the repository root:\n%s", displayPaths(root, problems))
}

func checkFrontmatter(root string) error {
	docsRoot := filepath.Join(root, "apps", "docs", "content", "docs")
	var problems []string
	err := filepath.WalkDir(docsRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".md" && ext != ".mdx" {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lines := strings.Split(string(raw), "\n")
		if len(lines) > 10 {
			lines = lines[:10]
		}
		header := strings.Join(lines, "\n")
		if !hasFrontmatterField(header, "title:") {
			problems = append(problems, relativePath(root, path)+": missing 'title:' frontmatter")
		}
		if !hasFrontmatterField(header, "description:") {
			problems = append(problems, relativePath(root, path)+": missing 'description:' frontmatter")
		}
		return nil
	})
	if err != nil {
		return err
	}
	if len(problems) == 0 {
		return nil
	}
	sort.Strings(problems)
	return errors.New(strings.Join(problems, "\n"))
}

func hasFrontmatterField(header, field string) bool {
	for _, line := range strings.Split(header, "\n") {
		if strings.HasPrefix(strings.TrimSuffix(line, "\r"), field) {
			return true
		}
	}
	return false
}

func checkGoFormat(root string) error {
	var changed []string
	for _, relativeRoot := range []string{"packages/kernel", "packages/cli"} {
		base := filepath.Join(root, filepath.FromSlash(relativeRoot))
		err := filepath.WalkDir(base, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			if filepath.Ext(path) != ".go" {
				return nil
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			normalized := bytes.ReplaceAll(raw, []byte("\r\n"), []byte("\n"))
			formatted, err := format.Source(normalized)
			if err != nil {
				return fmt.Errorf("format %s: %w", relativePath(root, path), err)
			}
			if !bytes.Equal(normalized, formatted) {
				changed = append(changed, path)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	if len(changed) == 0 {
		return nil
	}
	sort.Strings(changed)
	return fmt.Errorf("files need gofmt — run 'task fmt' first:\n%s", displayPaths(root, changed))
}

func displayPaths(root string, paths []string) string {
	display := make([]string, 0, len(paths))
	for _, path := range paths {
		display = append(display, relativePath(root, path))
	}
	return strings.Join(display, "\n")
}

func relativePath(root, path string) string {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(relative)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
