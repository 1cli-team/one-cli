//go:build windows

package secrets

import "testing"

func TestMergeIntoEnvironTreatsPathCaseInsensitivelyOnWindows(t *testing.T) {
	got := MergeIntoEnviron(
		[]string{"Path=C:\\Windows", "KEEP=yes"},
		map[string]string{"PATH": "C:\\tools"},
		true,
	)
	var paths []string
	for _, entry := range got {
		key := entry
		for i, char := range entry {
			if char == '=' {
				key = entry[:i]
				break
			}
		}
		if environmentKey(key) == "PATH" {
			paths = append(paths, entry)
		}
	}
	if len(paths) != 1 || paths[0] != "PATH=C:\\tools" {
		t.Fatalf("Windows PATH merge = %v; full env = %v", paths, got)
	}
}
