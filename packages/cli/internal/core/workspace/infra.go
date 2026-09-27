package workspace

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ResolveProjectWorkflowPath returns the canonical
// .github/workflows/ci-<id>.yml path for a project. Used by status to
// report whether each project has a workflow on disk.
func ResolveProjectWorkflowPath(projectRoot, targetDir string) string {
	rel, err := filepath.Rel(projectRoot, targetDir)
	if err != nil {
		rel = targetDir
	}
	rel = ToPosixPath(rel)
	id := workflowIDFromRelativeDir(rel)
	return filepath.Join(projectRoot, ".github", "workflows", "ci-"+id+".yml")
}

// HasProjectWorkflow reports whether the project's workflow file exists.
func HasProjectWorkflow(projectRoot, targetDir string) bool {
	_, err := os.Stat(ResolveProjectWorkflowPath(projectRoot, targetDir))
	return err == nil && !errors.Is(err, fs.ErrNotExist)
}

var (
	pathSeparators  = regexp.MustCompile(`[\\/]+`)
	nonAllowedChars = regexp.MustCompile(`[^a-zA-Z0-9._-]`)
	repeatedDashes  = regexp.MustCompile(`-+`)
)

func workflowIDFromRelativeDir(rel string) string {
	id := pathSeparators.ReplaceAllString(rel, "-")
	id = nonAllowedChars.ReplaceAllString(id, "-")
	id = repeatedDashes.ReplaceAllString(id, "-")
	return strings.ToLower(id)
}
