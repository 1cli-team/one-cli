//go:build windows

package runcmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAugmentPathForRunReplacesMixedCaseWindowsPath(t *testing.T) {
	projectRoot := filepath.Join("C:\\repo", "workspace")
	targetDir := filepath.Join(projectRoot, "apps", "web")
	got := augmentPathForRun([]string{"Path=C:\\Windows", "KEEP=yes"}, projectRoot, targetDir)

	var paths []string
	for _, entry := range got {
		key, _, found := strings.Cut(entry, "=")
		if found && isPathEnvironmentKey(key) {
			paths = append(paths, entry)
		}
	}
	if len(paths) != 1 {
		t.Fatalf("PATH entries = %v; full env = %v", paths, got)
	}
	wantPrefix := "PATH=" + filepath.Join(targetDir, "node_modules", ".bin") +
		string(os.PathListSeparator) + filepath.Join(projectRoot, "node_modules", ".bin") +
		string(os.PathListSeparator)
	if !strings.HasPrefix(paths[0], wantPrefix) || !strings.HasSuffix(paths[0], "C:\\Windows") {
		t.Fatalf("augmented PATH = %q, want prefix %q and inherited suffix", paths[0], wantPrefix)
	}
}
