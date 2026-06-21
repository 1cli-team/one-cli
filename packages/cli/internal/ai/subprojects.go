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

// Markers for the workspace AGENTS.md sub-project index block. Distinct
// from the agents:index markers — this block is regenerated on every
// `one add` so the canonical entry keeps a compact project table.
const (
	subprojectsStart = "<!-- one subprojects:start -->"
	subprojectsEnd   = "<!-- one subprojects:end -->"
)

// UpdateSubprojectsIndex regenerates the <!-- one subprojects --> block
// in the workspace AGENTS.md. The block lists each sub-project with a
// markdown link to its central .one/agents project guide.
//
// Best-effort: if the workspace AGENTS.md doesn't exist (e.g. an older
// workspace created before this skeleton landed) or doesn't carry the
// markers, the function is a no-op. Hand-written content outside the
// markers is preserved verbatim.
func UpdateSubprojectsIndex(projectRoot string) error {
	m, err := workspace.ReadManifest(projectRoot)
	if err != nil {
		return err
	}
	return writeSubprojectsIndex(projectRoot, sortedManifestProjects(m))
}

func writeSubprojectsIndex(projectRoot string, subs []workspace.ManifestProject) error {
	path := filepath.Join(projectRoot, GuideFilename(ProviderCodex))
	current, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// No workspace AGENTS.md to update — older workspace or the
			// user deleted it. Don't recreate; that's the scaffold's job.
			return nil
		}
		return err
	}
	body := renderSubprojectsTable(subs)
	curStr := string(current)
	if !strings.Contains(curStr, subprojectsStart) || !strings.Contains(curStr, subprojectsEnd) {
		if strings.Contains(curStr, generatedStart) || strings.Contains(curStr, legacyGeneratedStart) {
			next := strings.TrimRight(curStr, "\n") + "\n\n## Sub-projects\n\n" +
				subprojectsStart + "\n" + body + "\n" + subprojectsEnd + "\n"
			return os.WriteFile(path, []byte(next), 0o644)
		}
		// File exists but lacks all One CLI markers (user customized
		// aggressively). Don't fight them — leave as-is.
		return nil
	}
	pattern := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(subprojectsStart) + `.*?` + regexp.QuoteMeta(subprojectsEnd))
	replacement := subprojectsStart + "\n" + body + "\n" + subprojectsEnd
	next := pattern.ReplaceAllString(curStr, replacement)
	if !strings.HasSuffix(next, "\n") {
		next += "\n"
	}
	return os.WriteFile(path, []byte(next), 0o644)
}

// renderSubprojectsTable produces the markdown body that goes between
// the subprojects markers. Empty workspaces get a hint pointing the
// agent at `one add`.
func renderSubprojectsTable(subs []workspace.ManifestProject) string {
	if len(subs) == 0 {
		return "*No sub-projects yet. Run* `one add <template-id> --name <name>` *to scaffold one.*"
	}
	sorted := make([]workspace.ManifestProject, len(subs))
	copy(sorted, subs)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].RelativeDir < sorted[j].RelativeDir
	})
	var b strings.Builder
	b.WriteString("| Project | Template | Guide |\n")
	b.WriteString("|---------|----------|-------|\n")
	for _, s := range sorted {
		guidePath := projectGuideRelPath(s.RelativeDir)
		guideLink := fmt.Sprintf("[`%s`](%s)", guidePath, guidePath)
		b.WriteString(fmt.Sprintf("| `%s` | `%s` | %s |\n",
			s.RelativeDir, s.TemplateID, guideLink))
	}
	return strings.TrimRight(b.String(), "\n")
}
