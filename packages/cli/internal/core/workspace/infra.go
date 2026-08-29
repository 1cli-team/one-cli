package workspace

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// HasComposeService reports whether docker-compose.yml at projectRoot
// declares a service named workloadName. Matches the canonical indented
// YAML key — no deep parse.
func HasComposeService(projectRoot, workloadName string) bool {
	composePath := filepath.Join(projectRoot, "docker-compose.yml")
	raw, err := os.ReadFile(composePath)
	if err != nil {
		return false
	}
	content := normalizeNewlines(string(raw))
	pattern := regexp.MustCompile(`(?m)^\s{2}` + regexp.QuoteMeta(workloadName) + `:\s*$`)
	return pattern.MatchString(content)
}

// HasK8sWorkload reports whether k8s/deployment.yaml at projectRoot contains
// a metadata.name pointing at workloadName.
func HasK8sWorkload(projectRoot, workloadName string) bool {
	deploymentPath := filepath.Join(projectRoot, "k8s", "deployment.yaml")
	raw, err := os.ReadFile(deploymentPath)
	if err != nil {
		return false
	}
	content := normalizeNewlines(string(raw))
	pattern := regexp.MustCompile(`metadata:\n\s+name:\s+` + regexp.QuoteMeta(workloadName) + `\b`)
	return pattern.MatchString(content)
}

// HasComposeFile reports whether docker-compose.yml exists at the project
// root (used to derive coverage.docker_compose.enabled).
func HasComposeFile(projectRoot string) bool {
	_, err := os.Stat(filepath.Join(projectRoot, "docker-compose.yml"))
	return err == nil
}

// HasK8sFile reports whether k8s/deployment.yaml exists.
func HasK8sFile(projectRoot string) bool {
	_, err := os.Stat(filepath.Join(projectRoot, "k8s", "deployment.yaml"))
	return err == nil
}

// HasKustomizeDir reports whether the workspace has a generated kustomize
// tree. This is separate from HasK8sFile: v0.5+ deploy/kustomize writes under
// kustomize/, while the older raw k8s path wrote k8s/deployment.yaml.
func HasKustomizeDir(projectRoot string) bool {
	st, err := os.Stat(filepath.Join(projectRoot, "kustomize"))
	return err == nil && st.IsDir()
}

// ExpectedKustomizeFiles returns the deploy/kustomize artifact set for one
// workload, relative to the workspace root.
func ExpectedKustomizeFiles(workloadName string) []string {
	return []string{
		"kustomize/base/" + workloadName + ".yaml",
		"kustomize/base/kustomization.yaml",
		"kustomize/overlays/dev/kustomization.yaml",
		"kustomize/overlays/prod/kustomization.yaml",
	}
}

// MissingKustomizeFiles returns the deploy/kustomize artifact paths that do
// not exist yet, relative to the workspace root.
func MissingKustomizeFiles(projectRoot, workloadName string) []string {
	missing := []string{}
	for _, rel := range ExpectedKustomizeFiles(workloadName) {
		if _, err := os.Stat(filepath.Join(projectRoot, filepath.FromSlash(rel))); err != nil {
			missing = append(missing, rel)
		}
	}
	return missing
}

// HasDockerfile is a thin wrapper for the per-project Dockerfile check.
func HasDockerfile(targetDir string) bool {
	_, err := os.Stat(filepath.Join(targetDir, "Dockerfile"))
	return err == nil
}

// ResolveWorkloadName prefers kebab-case of the project name, falling
// back to kebab-case of the target dir's basename.
func ResolveWorkloadName(projectName, targetDir string) string {
	if k := ToKebabCase(projectName); k != "" {
		return k
	}
	return ToKebabCase(filepath.Base(targetDir))
}

func normalizeNewlines(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}

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
