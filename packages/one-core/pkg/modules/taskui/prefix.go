package taskui

import (
	"hash/fnv"
	"slices"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// Allocate once, before either output writer starts. Canonical task identities
// share a color across aliases, stdout/stderr, partial lines and history.
func taskPrefixStyles(names []string) map[string]uv.Style {
	palette := [...]ansi.IndexedColor{75, 81, 114, 141, 221, 215, 211, 159, 183, 117, 154, 209}
	var used [len(palette)]int
	ordered := slices.Clone(names)
	slices.Sort(ordered)
	styles := make(map[string]uv.Style, len(names))
	for _, name := range ordered {
		if name == "" {
			continue
		}
		if _, exists := styles[name]; exists {
			continue
		}
		hash := fnv.New32a()
		_, _ = hash.Write([]byte(name))
		start := int(hash.Sum32() % uint32(len(palette)))
		index := start
		// Resolve collisions deterministically, reusing colors only after every
		// palette entry has been assigned. Log arrival order never affects color.
		for offset := 1; offset < len(palette); offset++ {
			candidate := (start + offset) % len(palette)
			if used[candidate] < used[index] {
				index = candidate
			}
		}
		used[index]++
		styles[name] = uv.Style{Fg: palette[index]}
	}
	return styles
}
