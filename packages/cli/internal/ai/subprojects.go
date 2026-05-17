package ai

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/torchstellar-team/one-cli/packages/cli/internal/workspace"
)

// Markers for the workspace CLAUDE.md sub-project index block. Distinct
// from the per-provider ai-guides markers — this block is regenerated on
// every `one add` regardless of ai.providers config, so even codex-only
// workspaces still get a usable CLAUDE.md index.
const (
	subprojectsStart = "<!-- one subprojects:start -->"
	subprojectsEnd   = "<!-- one subprojects:end -->"
)

// UpdateSubprojectsIndex regenerates the <!-- one subprojects --> block
// in the workspace CLAUDE.md. The block lists each sub-project with a
// markdown link to its per-template CLAUDE.md (which template.Render
// copies into the project dir).
//
// Best-effort: if the workspace CLAUDE.md doesn't exist (e.g. an older
// workspace created before this skeleton landed) or doesn't carry the
// markers, the function is a no-op. Hand-written content outside the
// markers is preserved verbatim.
//
// Independent from Refresh's per-provider rendering: callable even when
// ai.providers is empty or only contains codex.
func UpdateSubprojectsIndex(projectRoot string) error {
	rootDirs, err := workspace.ResolveRootDirs(projectRoot, nil)
	if err != nil {
		return err
	}
	subs, err := workspace.DiscoverProjects(projectRoot, rootDirs)
	if err != nil {
		return err
	}
	return writeSubprojectsIndex(projectRoot, subs)
}

func writeSubprojectsIndex(projectRoot string, subs []workspace.Project) error {
	path := filepath.Join(projectRoot, "CLAUDE.md")
	current, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// No workspace CLAUDE.md to update — older workspace or the
			// user deleted it. Don't recreate; that's the scaffold's job.
			return nil
		}
		return err
	}
	if !strings.Contains(string(current), subprojectsStart) || !strings.Contains(string(current), subprojectsEnd) {
		// File exists but lacks markers (user customized aggressively).
		// Don't fight them — leave as-is.
		return nil
	}
	body := renderSubprojectsTable(subs)
	pattern := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(subprojectsStart) + `.*?` + regexp.QuoteMeta(subprojectsEnd))
	replacement := subprojectsStart + "\n" + body + "\n" + subprojectsEnd
	next := pattern.ReplaceAllString(string(current), replacement)
	if !strings.HasSuffix(next, "\n") {
		next += "\n"
	}
	return os.WriteFile(path, []byte(next), 0o644)
}

// renderSubprojectsTable produces the markdown body that goes between
// the subprojects markers. Empty workspaces get a hint pointing the
// agent at `one add`.
func renderSubprojectsTable(subs []workspace.Project) string {
	if len(subs) == 0 {
		return "*No sub-projects yet. Run* `one add <template-id> --name <name>` *to scaffold one.*"
	}
	sorted := make([]workspace.Project, len(subs))
	copy(sorted, subs)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].RelativeDir < sorted[j].RelativeDir
	})
	var b strings.Builder
	b.WriteString("| Project | Template | Per-project guide |\n")
	b.WriteString("|---------|----------|-------------------|\n")
	for _, s := range sorted {
		guideLink := fmt.Sprintf("[`%[1]s/CLAUDE.md`](%[1]s/CLAUDE.md)", s.RelativeDir)
		b.WriteString(fmt.Sprintf("| `%s` | `%s` | %s |\n",
			s.RelativeDir, s.TemplateID, guideLink))
	}
	return strings.TrimRight(b.String(), "\n")
}
